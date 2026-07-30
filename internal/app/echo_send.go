package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cocosip/dicom-cli/internal/config"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/network/client"
)

const verificationSOPClassUID = "1.2.840.10008.1.1"

// Instance is a source DICOM file prepared for C-STORE without transcoding it.
type Instance struct {
	Path           string
	Dataset        *dataset.Dataset
	SOPClass       string
	TransferSyntax string
}

// Session owns a reusable go-dicom Association.
type Session struct {
	client *client.Client
}

// SendOptions controls C-STORE batching, concurrency, retries, and stopping.
type SendOptions struct {
	MaxInstances int
	Concurrency  int
	Retries      int
	FailFast     bool
}

// SendFileResult records the final result for one source file.
type SendFileResult struct {
	Path     string `json:"path"`
	Attempts int    `json:"attempts"`
	Error    string `json:"error,omitempty"`
}

// SendReport records input handling and per-file C-STORE results.
type SendReport struct {
	Scanned   int              `json:"scanned"`
	Processed int              `json:"processed"`
	Skipped   int              `json:"skipped"`
	Failed    int              `json:"failed"`
	Skips     []files.Entry    `json:"skips,omitempty"`
	Files     []SendFileResult `json:"files"`
}

// ReadInstance parses a DICOM file and retains the source transfer syntax for
// Association negotiation. It never rewrites the input file or its dataset.
func ReadInstance(path string) (Instance, error) {
	parsed, err := parser.ParseFile(path)
	if err != nil {
		return Instance{}, err
	}
	sopClass, ok := parsed.Dataset.GetString(tag.SOPClassUID)
	if !ok || sopClass == "" {
		return Instance{}, fmt.Errorf("SOPClassUID is required for C-STORE")
	}
	if _, ok := parsed.Dataset.GetString(tag.SOPInstanceUID); !ok {
		return Instance{}, fmt.Errorf("SOPInstanceUID is required for C-STORE")
	}
	if parsed.TransferSyntax == nil {
		return Instance{}, fmt.Errorf("source transfer syntax is required for C-STORE")
	}
	return Instance{
		Path:           path,
		Dataset:        parsed.Dataset,
		SOPClass:       sopClass,
		TransferSyntax: parsed.TransferSyntax.UID().String(),
	}, nil
}

// Echo opens a Verification Association and sends one C-ECHO request.
func Echo(ctx context.Context, target config.PACSTarget) error {
	c := newClient(target)
	c.AddPresentationContext(
		verificationSOPClassUID,
		transfer.ImplicitVRLittleEndian.UID().String(),
		transfer.ExplicitVRLittleEndian.UID().String(),
	)
	if err := c.Connect(ctx, target.Host, target.Port); err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	return c.CEcho(ctx)
}

// Open negotiates a C-STORE Association for exactly the source SOP classes and
// transfer syntaxes supplied by instances. It adds no alternate syntax and
// therefore never authorizes an implicit transcode.
func Open(ctx context.Context, target config.PACSTarget, instances []Instance) (*Session, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("at least one DICOM instance is required")
	}
	contexts := make(map[string][]string)
	for _, instance := range instances {
		if instance.Dataset == nil || instance.SOPClass == "" || instance.TransferSyntax == "" {
			return nil, fmt.Errorf("invalid C-STORE instance %q", instance.Path)
		}
		if !contains(contexts[instance.SOPClass], instance.TransferSyntax) {
			contexts[instance.SOPClass] = append(contexts[instance.SOPClass], instance.TransferSyntax)
		}
	}
	c := newClient(target)
	for sopClass, syntaxes := range contexts {
		c.AddPresentationContext(sopClass, syntaxes...)
	}
	if err := c.Connect(ctx, target.Host, target.Port); err != nil {
		return nil, err
	}
	return &Session{client: c}, nil
}

// Store sends an instance through the reusable Association.
func (session *Session) Store(ctx context.Context, instance Instance) error {
	if session == nil || session.client == nil {
		return fmt.Errorf("DIMSE session is not connected")
	}
	return session.client.CStore(ctx, instance.Dataset)
}

// Close releases the Association.
func (session *Session) Close() error {
	if session == nil || session.client == nil {
		return nil
	}
	return session.client.Close()
}

// Retryable reports whether an error was caused by a transport interruption or
// timeout. A remote C-STORE status is a completed PACS response and must never
// be retried automatically.
func Retryable(err error) bool {
	if err == nil || strings.Contains(err.Error(), "C-STORE failed with status") {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError)
}

// Send parses selected files, sends them through reusable Associations, and
// writes per-file progress to progress. DICOM wire operations are delegated to
// go-dicom's network/client package through Open and Session.Store.
func Send(ctx context.Context, progress io.Writer, target config.PACSTarget, entries []files.Entry, options SendOptions) SendReport {
	report := SendReport{Scanned: len(entries)}
	var instances []Instance
	for _, entry := range entries {
		if entry.Skipped {
			report.Skipped++
			report.Skips = append(report.Skips, entry)
			continue
		}
		instance, err := ReadInstance(entry.Path)
		if err != nil {
			report.Failed++
			report.Files = append(report.Files, SendFileResult{Path: entry.Path, Error: err.Error()})
			if options.FailFast {
				return report
			}
			continue
		}
		instances = append(instances, instance)
	}
	groups := splitInstances(instances, options.MaxInstances)
	if options.FailFast {
		for _, group := range groups {
			for _, result := range sendGroup(ctx, progress, target, group, options.Retries) {
				report.Files = append(report.Files, result)
				if result.Error != "" {
					report.Failed++
					return report
				}
				report.Processed++
			}
		}
		return report
	}
	jobs := make(chan []Instance)
	results := make(chan SendFileResult, len(instances))
	var workers sync.WaitGroup
	for range options.Concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for group := range jobs {
				for _, result := range sendGroup(ctx, progress, target, group, options.Retries) {
					results <- result
				}
			}
		}()
	}
	go func() {
		for _, group := range groups {
			jobs <- group
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()
	for result := range results {
		report.Files = append(report.Files, result)
		if result.Error != "" {
			report.Failed++
		} else {
			report.Processed++
		}
	}
	return report
}

func splitInstances(instances []Instance, maxInstances int) [][]Instance {
	if maxInstances <= 0 || maxInstances >= len(instances) {
		return [][]Instance{instances}
	}
	groups := make([][]Instance, 0, (len(instances)+maxInstances-1)/maxInstances)
	for len(instances) > 0 {
		end := maxInstances
		if end > len(instances) {
			end = len(instances)
		}
		groups = append(groups, instances[:end])
		instances = instances[end:]
	}
	return groups
}

func sendGroup(ctx context.Context, progress io.Writer, target config.PACSTarget, group []Instance, retries int) []SendFileResult {
	session, err := Open(ctx, target, group)
	if err != nil {
		results := make([]SendFileResult, 0, len(group))
		for _, instance := range group {
			results = append(results, sendWithRetries(ctx, progress, target, instance, retries, err))
		}
		return results
	}
	defer func() { _ = session.Close() }()
	results := make([]SendFileResult, 0, len(group))
	for _, instance := range group {
		err := session.Store(ctx, instance)
		if err == nil {
			_, _ = fmt.Fprintf(progress, "sent %s\n", instance.Path)
			results = append(results, SendFileResult{Path: instance.Path, Attempts: 1})
			continue
		}
		results = append(results, sendWithRetries(ctx, progress, target, instance, retries, err))
	}
	return results
}

func sendWithRetries(ctx context.Context, progress io.Writer, target config.PACSTarget, instance Instance, retries int, initial error) SendFileResult {
	err := initial
	attempts := 1
	for Retryable(err) && attempts <= retries {
		attempts++
		_, _ = fmt.Fprintf(progress, "retrying %s attempt=%d\n", instance.Path, attempts)
		session, openErr := Open(ctx, target, []Instance{instance})
		if openErr == nil {
			err = session.Store(ctx, instance)
			_ = session.Close()
		} else {
			err = openErr
		}
	}
	if err == nil {
		_, _ = fmt.Fprintf(progress, "sent %s\n", instance.Path)
		return SendFileResult{Path: instance.Path, Attempts: attempts}
	}
	_, _ = fmt.Fprintf(progress, "failed %s: %v\n", instance.Path, err)
	return SendFileResult{Path: instance.Path, Attempts: attempts, Error: err.Error()}
}

func newClient(target config.PACSTarget) *client.Client {
	timeouts := target.Timeouts
	if timeouts.Connect <= 0 {
		timeouts.Connect = 10 * time.Second
	}
	if timeouts.Associate <= 0 {
		timeouts.Associate = 30 * time.Second
	}
	if timeouts.Idle <= 0 {
		timeouts.Idle = 5 * time.Minute
	}
	return client.New(
		client.WithCallingAE(target.CallingAETitle),
		client.WithCalledAE(target.CalledAETitle),
		client.WithConnectTimeout(timeouts.Connect),
		client.WithAssociationTimeout(timeouts.Associate),
		client.WithRequestTimeout(timeouts.Idle),
	)
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
