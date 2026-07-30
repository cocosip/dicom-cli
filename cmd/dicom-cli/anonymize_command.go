package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"

	anonymizepkg "github.com/cocosip/dicom-cli/internal/anonymize"
	"github.com/cocosip/dicom-cli/internal/app"
	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/dicom-cli/internal/rules"
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
	Input   string                `json:"input"`
	Output  string                `json:"output"`
	Changes []anonymizepkg.Change `json:"changes"`
}

func newAnonymizeCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	var profile, destination, reportPath, filter string
	var profileOptions []string
	var recursive, failFast, flatten, asJSON bool
	command := &cobra.Command{
		Use:   "anonymize <file-or-directory>",
		Short: "Anonymize DICOM files into new files",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if profile == "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--profile is required"))
			}
			selected, condition, err := loadAnonymizeProfile(runtime, root.rulesPath, profile, filter)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			profileOptions = append(profileOptions, selected.Options...)
			engine, err := anonymizepkg.NewEngine(anonymizepkg.Options{ProfileOptions: profileOptions, Rules: selected.Rules})
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
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
			entries, err := files.Scan(input, recursive, func(path string) (bool, string, error) {
				if condition == nil {
					return true, "", nil
				}
				parsed, parseErr := parser.ParseFile(path)
				if parseErr != nil {
					return true, "", nil
				}
				if matchCondition(parsed.Dataset, *condition) {
					return true, "", nil
				}
				return false, "filter did not match", nil
			})
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
				result, err := engine.Anonymize(parsed.Dataset)
				if err != nil {
					return err
				}
				outputPath := "-"
				if destination == "-" {
					if err := writer.Write(runtime.Stdout, result.Dataset, writer.WithTransferSyntax(parsed.TransferSyntax)); err != nil {
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
					if err := writer.WriteFile(outputPath, result.Dataset, writer.WithTransferSyntax(parsed.TransferSyntax)); err != nil {
						return err
					}
				}
				report.Files = append(report.Files, anonymizeFileReport{Input: path, Output: outputPath, Changes: result.Changes})
				return nil
			})
			report.UIDMappings = engine.UIDMappings()
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
				if err := writeAnonymizeSummary(runtime.Stderr, result, asJSON); err != nil {
					return err
				}
			} else if err := writeAnonymizeSummary(runtime.Stdout, result, asJSON); err != nil {
				return err
			}
			if summary.Failed > 0 {
				return apperr.Wrap(apperr.KindOperation, fmt.Errorf("anonymize failed for %d file(s)", summary.Failed))
			}
			return nil
		},
	}
	command.Flags().StringVarP(&profile, "profile", "p", "", "basic or rules anonymize profile")
	command.Flags().StringArrayVar(&profileOptions, "option", nil, "standard anonymize profile option")
	command.Flags().StringVar(&filter, "filter", "", "named rules filter")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "scan subdirectories")
	command.Flags().BoolVar(&failFast, "fail-fast", false, "stop after the first file failure")
	command.Flags().BoolVar(&flatten, "flatten", false, "do not preserve input directory structure")
	command.Flags().StringVarP(&destination, "output", "o", "", "DICOM output directory or - for one file")
	command.Flags().BoolVarP(&asJSON, "json", "j", false, "write JSON summary")
	command.Flags().StringVar(&reportPath, "report", "", "write detailed sensitive report to file")
	return command
}

func loadAnonymizeProfile(runtime Runtime, configuredPath, name, requestedFilter string) (rules.AnonymizeProfile, *rules.Condition, error) {
	profile := rules.AnonymizeProfile{}
	if name == "basic" && requestedFilter == "" {
		return profile, nil, nil
	}
	path, err := rulesPath(runtime, configuredPath, nil)
	if err != nil {
		return rules.AnonymizeProfile{}, nil, err
	}
	file, err := rules.Load(path)
	if err != nil {
		return rules.AnonymizeProfile{}, nil, err
	}
	if name != "basic" {
		var ok bool
		profile, ok = file.Anonymize.Profiles[name]
		if !ok {
			return rules.AnonymizeProfile{}, nil, fmt.Errorf("anonymize profile %q does not exist", name)
		}
	}
	filterName := requestedFilter
	if filterName == "" {
		filterName = profile.Filter
	}
	if filterName == "" {
		return profile, nil, nil
	}
	condition, ok := file.Filters[filterName]
	if !ok {
		return rules.AnonymizeProfile{}, nil, fmt.Errorf("anonymize filter %q does not exist", filterName)
	}
	return profile, &condition, nil
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

func writeAnonymizeSummary(writer io.Writer, summary anonymizeSummary, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(writer).Encode(summary)
	}
	_, err := fmt.Fprintf(writer, "scanned=%d processed=%d skipped=%d failed=%d\n", summary.Scanned, summary.Processed, summary.Skipped, summary.Failed)
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

func matchCondition(ds *dataset.Dataset, condition rules.Condition) bool {
	if len(condition.All) > 0 {
		for _, child := range condition.All {
			if !matchCondition(ds, child) {
				return false
			}
		}
		return true
	}
	if len(condition.Any) > 0 {
		for _, child := range condition.Any {
			if matchCondition(ds, child) {
				return true
			}
		}
		return false
	}
	elem, err := dicom.ResolveElement(ds, condition.Path)
	if condition.Exists != nil {
		return err == nil == *condition.Exists
	}
	if err != nil {
		return false
	}
	value := elementValue(elem)
	if condition.Equals != nil {
		return value == *condition.Equals
	}
	if condition.Matches != "" {
		matched, compileErr := regexp.MatchString(condition.Matches, value)
		return compileErr == nil && matched
	}
	if condition.Range != nil {
		number, parseErr := strconv.ParseFloat(value, 64)
		return parseErr == nil && (condition.Range.Min == nil || number >= *condition.Range.Min) && (condition.Range.Max == nil || number <= *condition.Range.Max)
	}
	return false
}

func elementValue(elem element.Element) string {
	if value, ok := elem.(*element.String); ok {
		return value.GetString()
	}
	return elem.String()
}
