package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	convertpkg "github.com/cocosip/dicom-cli/internal/convert"
	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

func newTranscodeCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	var target, destination, filterName string
	var recursive, failFast, flatten bool
	command := &cobra.Command{
		Use:   "transcode <file>",
		Short: "Re-encode DICOM transfer syntaxes",
		Long: "Re-encode DICOM transfer syntaxes.\n\n" +
			"--to accepts a transfer syntax alias or standard UID. Run " +
			"`dicom-cli transcode formats` to list values available in this binary.",
		Example: "  dicom-cli transcode formats\n" +
			"  dicom-cli transcode --to rle --output compressed.dcm image.dcm\n" +
			"  dicom-cli transcode --to 1.2.840.10008.1.2.1 --output output.dcm image.dcm",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if target == "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--to is required"))
			}
			if destination == "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--output is required"))
			}
			if filepath.Clean(args[0]) == filepath.Clean(destination) {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("output path is the input path"))
			}
			format, err := dicom.ResolveTransferSyntax(target)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			inputInfo, err := os.Stat(args[0])
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if inputInfo.IsDir() {
				condition, err := loadTranscodeFilter(runtime, root.rulesPath, filterName)
				if err != nil {
					return apperr.Wrap(apperr.KindInput, err)
				}
				entries, err := files.Scan(args[0], recursive, func(path string) (bool, string, error) {
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
				failed := 0
				for _, entry := range entries {
					if entry.Skipped {
						continue
					}
					outputPath, pathErr := files.OutputPath(entry.Path, args[0], destination, !flatten)
					if pathErr == nil {
						pathErr = transcodeFile(entry.Path, outputPath, format.Syntax)
					}
					if pathErr != nil {
						failed++
						if failFast {
							break
						}
					}
				}
				if failed > 0 {
					return apperr.Wrap(apperr.KindOperation, fmt.Errorf("transcode failed for %d file(s)", failed))
				}
				return nil
			}
			if _, err := os.Stat(destination); err == nil {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("output file %q already exists", destination))
			} else if !os.IsNotExist(err) {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if err := transcodeFile(args[0], destination, format.Syntax); err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&target, "to", "", "target transfer syntax alias or UID; run 'transcode formats' to list values")
	command.Flags().StringVarP(&destination, "output", "o", "", "output DICOM file")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "scan subdirectories")
	command.Flags().BoolVar(&failFast, "fail-fast", false, "stop after the first file failure")
	command.Flags().BoolVar(&flatten, "flatten", false, "do not preserve input directory structure")
	command.Flags().StringVar(&filterName, "filter", "", "named rules filter for directory input")
	var asJSON bool
	formats := &cobra.Command{
		Use:   "formats",
		Short: "List transfer syntaxes available in this binary",
		Args:  noArgs,
		RunE: func(*cobra.Command, []string) error {
			available := dicom.RuntimeCodecs()
			if asJSON {
				return json.NewEncoder(runtime.Stdout).Encode(available)
			}
			for _, format := range available {
				marker := ""
				if format.Experimental {
					marker = " experimental"
				}
				if _, err := fmt.Fprintf(runtime.Stdout, "%s\t%s\tencode=%t\tdecode=%t%s\n", format.Alias, format.UID, format.Encode, format.Decode, marker); err != nil {
					return err
				}
			}
			return nil
		},
	}
	formats.Flags().BoolVarP(&asJSON, "json", "j", false, "write JSON output")
	command.AddCommand(formats)
	return command
}

func loadTranscodeFilter(runtime Runtime, configuredRules, name string) (*rules.Condition, error) {
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
		return nil, fmt.Errorf("transcode filter %q does not exist", name)
	}
	return &condition, nil
}

func transcodeFile(input, destination string, syntax *transfer.Syntax) error {
	parsed, err := parser.ParseFile(input)
	if err != nil {
		return err
	}
	converted, err := convertpkg.Transcode(parsed.Dataset, parsed.TransferSyntax, syntax)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return writer.WriteFile(destination, converted, writer.WithTransferSyntax(syntax))
}
