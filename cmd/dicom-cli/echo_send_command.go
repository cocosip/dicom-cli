package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/app"
	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/config"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/dicom-cli/internal/i18n"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

type dimseOptions struct {
	target, host, callingAE, calledAE             string
	port                                          int
	connectTimeout, associateTimeout, idleTimeout time.Duration
}

func newEchoCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("echo")
	options := dimseOptions{}
	var asJSON bool
	command := &cobra.Command{
		Use:   "echo",
		Short: text.Short,
		Long:  text.Long,
		Example: "  dicom-cli echo --target local-pacs\n" +
			"  dicom-cli echo --host pacs.example.test --port 11112 --calling-ae DICOMCLI --called-ae PACS",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			target, err := loadDIMSETarget(command, runtime, root, options)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if err := app.Echo(context.Background(), target); err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if asJSON {
				return json.NewEncoder(runtime.Stdout).Encode(map[string]any{"host": target.Host, "port": target.Port, "status": "success"})
			}
			_, err = fmt.Fprintln(runtime.Stdout, root.localizer.Text(i18n.EchoSucceeded, target.Host, target.Port))
			return err
		},
	}
	localizedHelpFlag(command, root.localizer)
	bindDIMSEFlags(command, &options, root.localizer)
	command.Flags().BoolVarP(&asJSON, "json", "j", false, root.localizer.FlagUsage("json", "write JSON result"))
	return command
}

func newSendCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("send")
	options := dimseOptions{}
	var recursive, failFast, asJSON bool
	var filter, reportPath, failedListPath string
	var maxInstances, concurrency, retries int
	command := &cobra.Command{
		Use:   "send <file-or-directory-or->",
		Short: text.Short,
		Long:  text.Long,
		Example: "  dicom-cli send --target local-pacs image.dcm\n" +
			"  dicom-cli send --target local-pacs --recursive study\n" +
			"  Get-Content failed-paths.txt | dicom-cli send --target local-pacs -",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			target, err := loadDIMSETarget(command, runtime, root, options)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if maxInstances < 0 || concurrency < 1 || retries < 0 {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--max-instances must be non-negative, --concurrency must be positive, and --retries must be non-negative"))
			}
			condition, err := loadSendFilter(runtime, root.rulesPath, filter)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			entries, err := collectSendEntries(runtime.Stdin, args[0], recursive, condition)
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			report := app.Send(context.Background(), runtime.Stderr, root.localizer, target, entries, app.SendOptions{
				MaxInstances: maxInstances,
				Concurrency:  concurrency,
				Retries:      retries,
				FailFast:     failFast,
			})
			if reportPath != "" {
				if err := writeSendJSON(reportPath, report); err != nil {
					return apperr.Wrap(apperr.KindOperation, err)
				}
			}
			if failedListPath != "" {
				if err := writeFailedList(failedListPath, report.Files); err != nil {
					return apperr.Wrap(apperr.KindOperation, err)
				}
			}
			if asJSON {
				err = json.NewEncoder(runtime.Stdout).Encode(report)
			} else {
				_, err = fmt.Fprintln(runtime.Stdout, root.localizer.BatchSummary(report.Scanned, report.Processed, report.Skipped, report.Failed))
			}
			if err != nil {
				return err
			}
			if report.Failed > 0 {
				return apperr.Wrap(apperr.KindOperation, fmt.Errorf("send failed for %d file(s)", report.Failed))
			}
			return nil
		},
	}
	localizedHelpFlag(command, root.localizer)
	bindDIMSEFlags(command, &options, root.localizer)
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, root.localizer.FlagUsage("recursive", "scan subdirectories"))
	command.Flags().BoolVar(&failFast, "fail-fast", false, root.localizer.FlagUsage("fail-fast", "stop after the first file failure"))
	command.Flags().StringVar(&filter, "filter", "", root.localizer.FlagUsage("filter", "named rules filter for directory input"))
	command.Flags().IntVar(&maxInstances, "max-instances", 0, root.localizer.FlagUsage("max-instances", "maximum instances per Association (0 is unlimited)"))
	command.Flags().IntVar(&concurrency, "concurrency", 1, root.localizer.FlagUsage("concurrency", "maximum concurrent Associations"))
	command.Flags().IntVar(&retries, "retries", 1, root.localizer.FlagUsage("retries", "retries for network and timeout failures"))
	command.Flags().StringVar(&reportPath, "report", "", root.localizer.FlagUsage("report", "write detailed JSON report"))
	command.Flags().StringVar(&failedListPath, "failed-list", "", root.localizer.FlagUsage("failed-list", "write failed paths as a newline-delimited list"))
	command.Flags().BoolVarP(&asJSON, "json", "j", false, root.localizer.FlagUsage("json", "write JSON summary"))
	return command
}

func bindDIMSEFlags(command *cobra.Command, options *dimseOptions, localizer i18n.Localizer) {
	flags := command.Flags()
	flags.StringVarP(&options.target, "target", "t", "", localizer.FlagUsage("target", "named PACS target"))
	flags.StringVar(&options.host, "host", "", localizer.FlagUsage("host", "PACS host override"))
	flags.IntVar(&options.port, "port", 0, localizer.FlagUsage("port", "PACS port override"))
	flags.StringVar(&options.callingAE, "calling-ae", "", localizer.FlagUsage("calling-ae", "calling AE Title override"))
	flags.StringVar(&options.calledAE, "called-ae", "", localizer.FlagUsage("called-ae", "called AE Title override"))
	flags.DurationVar(&options.connectTimeout, "connect-timeout", 0, localizer.FlagUsage("connect-timeout", "TCP connection timeout"))
	flags.DurationVar(&options.associateTimeout, "associate-timeout", 0, localizer.FlagUsage("associate-timeout", "Association negotiation timeout"))
	flags.DurationVar(&options.idleTimeout, "idle-timeout", 0, localizer.FlagUsage("idle-timeout", "DIMSE read/write idle timeout"))
}

func loadDIMSETarget(command *cobra.Command, runtime Runtime, root *rootOptions, options dimseOptions) (config.PACSTarget, error) {
	workingDirectory, err := runtime.Getwd()
	if err != nil {
		return config.PACSTarget{}, err
	}
	userConfigDirectory, err := runtime.UserConfigDir()
	if err != nil {
		return config.PACSTarget{}, err
	}
	loaded, _, err := config.Load(config.LocateOptions{Path: root.configPath, WorkingDir: workingDirectory, UserConfigDir: userConfigDirectory, LookupEnv: runtime.LookupEnv})
	if err != nil {
		return config.PACSTarget{}, err
	}
	overrides := config.TargetOverrides{
		Target:           changedString(command, "target", options.target),
		Host:             changedString(command, "host", options.host),
		Port:             changedInt(command, "port", options.port),
		CallingAETitle:   changedString(command, "calling-ae", options.callingAE),
		CalledAETitle:    changedString(command, "called-ae", options.calledAE),
		ConnectTimeout:   changedDuration(command, "connect-timeout", options.connectTimeout),
		AssociateTimeout: changedDuration(command, "associate-timeout", options.associateTimeout),
		IdleTimeout:      changedDuration(command, "idle-timeout", options.idleTimeout),
	}
	target, err := config.ResolveTarget(loaded, overrides, runtime.LookupEnv)
	if err != nil {
		return config.PACSTarget{}, err
	}
	if target.Host == "" || target.Port == 0 || target.CallingAETitle == "" || target.CalledAETitle == "" {
		return config.PACSTarget{}, fmt.Errorf("effective target requires host, port, calling AE Title, and called AE Title")
	}
	return target, nil
}

func changedString(command *cobra.Command, name, value string) *string {
	if command.Flags().Changed(name) {
		return &value
	}
	return nil
}

func changedInt(command *cobra.Command, name string, value int) *int {
	if command.Flags().Changed(name) {
		return &value
	}
	return nil
}

func changedDuration(command *cobra.Command, name string, value time.Duration) *time.Duration {
	if command.Flags().Changed(name) {
		return &value
	}
	return nil
}

func loadSendFilter(runtime Runtime, configuredRules, name string) (*rules.Condition, error) {
	if name == "" {
		return nil, nil
	}
	path, err := rulesPath(runtime, configuredRules, nil)
	if err != nil {
		return nil, err
	}
	file, err := rules.Load(path)
	if err != nil {
		return nil, err
	}
	condition, ok := file.Filters[name]
	if !ok {
		return nil, fmt.Errorf("send filter %q does not exist", name)
	}
	return &condition, nil
}

func collectSendEntries(stdin io.Reader, input string, recursive bool, condition *rules.Condition) ([]files.Entry, error) {
	if input == "-" {
		return collectStdinPaths(stdin)
	}
	return files.Scan(input, recursive, func(path string) (bool, string, error) {
		if condition == nil {
			return true, "", nil
		}
		parsed, err := parser.ParseFile(path)
		if err != nil {
			return true, "", nil
		}
		if matchCondition(parsed.Dataset, *condition) {
			return true, "", nil
		}
		return false, "filter did not match", nil
	})
}

func collectStdinPaths(stdin io.Reader) ([]files.Entry, error) {
	var entries []files.Entry
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path != "" {
			entries = append(entries, files.Entry{Path: path})
		}
	}
	return entries, scanner.Err()
}

func writeSendJSON(path string, report app.SendReport) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("report file %q already exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}

func writeFailedList(path string, results []app.SendFileResult) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("failed list %q already exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	var paths []string
	for _, result := range results {
		if result.Error != "" {
			paths = append(paths, result.Path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(paths, "\n")), 0o600)
}
