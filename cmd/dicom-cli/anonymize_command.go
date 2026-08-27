package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/app"
	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/dicom-cli/internal/i18n"
	goanonymizer "github.com/cocosip/go-dicom/pkg/dicom/anonymizer"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

type anonymizeSummary struct {
	Scanned   int           `json:"scanned"`
	Processed int           `json:"processed"`
	Skipped   int           `json:"skipped"`
	Failed    int           `json:"failed"`
	Skips     []files.Entry `json:"skips,omitempty"`
}

type anonymizeReport struct {
	Files       []anonymizeFileReport `json:"files"`
	UIDMappings map[string]string     `json:"uid_mappings"`
}

type anonymizeFileReport struct {
	Input   string            `json:"input"`
	Output  string            `json:"output"`
	Changes []anonymizeChange `json:"changes"`
}

type anonymizeChange struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func newAnonymizeCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("anonymize")
	var destination, reportPath string
	var profileOptions []string
	var recursive, failFast, flatten, asJSON bool
	command := &cobra.Command{
		Use:   "anonymize <file-or-directory>",
		Short: text.Short,
		Long:  text.Long,
		Example: "  dicom-cli anonymize --output anonymized image.dcm\n" +
			"  dicom-cli anonymize --option retain-uids --recursive --output anonymized study",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if root.rulesPath != "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("anonymize does not support --rules"))
			}
			options, err := parseAnonymizeProfileOptions(profileOptions)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			anonymizer := goanonymizer.NewAnonymizer(goanonymizer.NewSecurityProfile(goanonymizer.BasicProfile | options))
			input := args[0]
			info, err := os.Stat(input)
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if destination == "-" && info.IsDir() {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("binary stdout requires exactly one input file"))
			}
			if reportPath == "-" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--report must be a file path"))
			}
			outputRoot, err := anonymizeOutputRoot(runtime, input, destination)
			if err != nil {
				return err
			}
			singleOutput, err := anonymizeSingleOutput(input, destination, info.IsDir())
			if err != nil {
				return err
			}
			entries, err := files.Scan(input, recursive, func(string) (bool, string, error) { return true, "", nil })
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if destination == "-" && (len(entries) != 1 || entries[0].Skipped) {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("binary stdout requires exactly one selected result"))
			}
			report := anonymizeReport{}
			summary := app.Run(entries, failFast, func(path string) error {
				parsed, err := parser.ParseFile(path)
				if err != nil {
					return err
				}
				before := anonymizeSnapshot(parsed.Dataset)
				anonymized, err := anonymizer.Anonymize(parsed.Dataset)
				if err != nil {
					return err
				}
				outputPath := "-"
				if destination == "-" {
					if err := writer.Write(runtime.Stdout, anonymized, writer.WithTransferSyntax(parsed.TransferSyntax)); err != nil {
						return err
					}
				} else {
					if singleOutput != "" {
						outputPath = singleOutput
					} else {
						outputPath, err = files.OutputPath(path, input, outputRoot, info.IsDir() && !flatten)
						if err != nil {
							return err
						}
					}
					if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
						return err
					}
					if err := writer.WriteFile(outputPath, anonymized, writer.WithTransferSyntax(parsed.TransferSyntax)); err != nil {
						return err
					}
				}
				report.Files = append(report.Files, anonymizeFileReport{Input: path, Output: outputPath, Changes: anonymizeChanges(before, anonymizeSnapshot(anonymized))})
				return nil
			})
			report.UIDMappings = cloneAnonymizeUIDMappings(anonymizer.ReplacedUIDs)
			if reportPath != "" {
				content, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				if err := writeAnonymizeReport(reportPath, content); err != nil {
					return err
				}
			}
			result := anonymizeSummary{Scanned: summary.Scanned, Processed: summary.Processed, Skipped: summary.Skipped, Failed: summary.Failed, Skips: summary.Skips}
			if destination == "-" {
				if err := writeAnonymizeSummary(runtime.Stderr, result, asJSON, root.localizer); err != nil {
					return err
				}
			} else if err := writeAnonymizeSummary(runtime.Stdout, result, asJSON, root.localizer); err != nil {
				return err
			}
			if summary.Failed > 0 {
				return apperr.Wrap(apperr.KindOperation, fmt.Errorf("anonymize failed for %d file(s)", summary.Failed))
			}
			return nil
		},
	}
	localizedHelpFlag(command, root.localizer)
	command.Flags().StringArrayVar(&profileOptions, "option", nil, root.localizer.FlagUsage("option", "standard anonymize profile option"))
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, root.localizer.FlagUsage("recursive", "scan subdirectories"))
	command.Flags().BoolVar(&failFast, "fail-fast", false, root.localizer.FlagUsage("fail-fast", "stop after the first file failure"))
	command.Flags().BoolVar(&flatten, "flatten", false, root.localizer.FlagUsage("flatten", "do not preserve input directory structure"))
	command.Flags().StringVarP(&destination, "output", "o", "", root.localizer.FlagUsage("output", "DICOM output directory or - for one file"))
	command.Flags().BoolVarP(&asJSON, "json", "j", false, root.localizer.FlagUsage("json", "write JSON summary"))
	command.Flags().StringVar(&reportPath, "report", "", root.localizer.FlagUsage("report", "write detailed sensitive report to file"))
	return command
}

func anonymizeOutputRoot(runtime Runtime, input, destination string) (string, error) {
	if destination == "-" {
		return "", nil
	}
	if destination != "" {
		inputPath, err := filepath.Abs(input)
		if err != nil {
			return "", err
		}
		outputPath, err := filepath.Abs(destination)
		if err != nil {
			return "", err
		}
		if filepath.Clean(inputPath) == filepath.Clean(outputPath) {
			return "", apperr.Wrap(apperr.KindInput, fmt.Errorf("output path is the input path"))
		}
		return destination, nil
	}
	workingDirectory, err := runtime.Getwd()
	if err != nil {
		return "", err
	}
	return files.DefaultOutputDirectory(workingDirectory, "anonymize"), nil
}

func anonymizeSingleOutput(input, destination string, directoryInput bool) (string, error) {
	if directoryInput || destination == "" || destination == "-" {
		return "", nil
	}
	inputPath, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	outputPath, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if filepath.Clean(inputPath) == filepath.Clean(outputPath) {
		return "", apperr.Wrap(apperr.KindInput, fmt.Errorf("output path is the input path"))
	}
	if _, err := os.Stat(destination); err == nil {
		return "", apperr.Wrap(apperr.KindInput, fmt.Errorf("output file %q already exists", destination))
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return destination, nil
}

func writeAnonymizeSummary(writer io.Writer, summary anonymizeSummary, asJSON bool, localizer i18n.Localizer) error {
	if asJSON {
		return json.NewEncoder(writer).Encode(summary)
	}
	_, err := fmt.Fprintln(writer, localizer.BatchSummary(summary.Scanned, summary.Processed, summary.Skipped, summary.Failed))
	return err
}

func writeAnonymizeReport(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("report file %q already exists", path))
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}

var anonymizeProfileOptions = map[string]goanonymizer.SecurityProfileOptions{
	"retain-safe-private":                                          goanonymizer.RetainSafePrivate,
	"retain-uids":                                                  goanonymizer.RetainUIDs,
	"retain-device-identity":                                       goanonymizer.RetainDeviceIdent,
	"retain-institution-identity":                                  goanonymizer.RetainInstitutionIdent,
	"retain-patient-characteristics":                               goanonymizer.RetainPatientChars,
	"retain-longitudinal-temporal-information-with-full-dates":     goanonymizer.RetainLongFullDates,
	"retain-longitudinal-temporal-information-with-modified-dates": goanonymizer.RetainLongModifDates,
	"clean-descriptors":                                            goanonymizer.CleanDesc,
	"clean-structured-content":                                     goanonymizer.CleanStructdCont,
	"clean-graphics":                                               goanonymizer.CleanGraph,
}

func parseAnonymizeProfileOptions(names []string) (goanonymizer.SecurityProfileOptions, error) {
	var selected goanonymizer.SecurityProfileOptions
	for _, name := range names {
		option, ok := anonymizeProfileOptions[name]
		if !ok {
			return 0, fmt.Errorf("unsupported anonymize profile option %q", name)
		}
		selected |= option
	}
	return selected, nil
}

func anonymizeSnapshot(ds *dataset.Dataset) map[string]string {
	values := map[string]string{}
	collectAnonymizeValues(ds, "", values)
	return values
}

func collectAnonymizeValues(ds *dataset.Dataset, prefix string, values map[string]string) {
	for _, elem := range ds.Elements() {
		path := elem.Tag().String()
		if prefix != "" {
			path = prefix + "." + path
		}
		if sequence, ok := elem.(*dataset.Sequence); ok {
			values[path] = "sequence"
			for index := 0; index < sequence.Count(); index++ {
				collectAnonymizeValues(sequence.GetItem(index), fmt.Sprintf("%s[%d]", path, index), values)
			}
			continue
		}
		if stringElement, ok := elem.(*element.String); ok {
			values[path] = strings.Join(stringElement.GetValues(), "\\")
			continue
		}
		values[path] = fmt.Sprint(elem)
	}
}

func anonymizeChanges(before, after map[string]string) []anonymizeChange {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	paths := make([]string, 0, len(keys))
	for path := range keys {
		if before[path] != after[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	result := make([]anonymizeChange, 0, len(paths))
	for _, path := range paths {
		result = append(result, anonymizeChange{Path: path, Before: before[path], After: after[path]})
	}
	return result
}

func cloneAnonymizeUIDMappings(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for original, replacement := range values {
		result[original] = replacement
	}
	return result
}
