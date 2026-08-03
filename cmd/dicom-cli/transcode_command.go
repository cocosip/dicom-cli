package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	text := root.localizer.Command("transcode")
	var input, target, destination, filterName string
	var recursive, failFast, flatten bool
	command := &cobra.Command{
		Use:   "transcode --input <file-or-directory>",
		Short: text.Short,
		Long:  transcodeHelpText(text.Long, root.localizer.IsChineseSimplified()),
		Example: "  dicom-cli transcode formats\n" +
			"  dicom-cli transcode --input image.dcm --to rle --output compressed.dcm\n" +
			"  dicom-cli transcode --input study --recursive --to 1.2.840.10008.1.2.1 --output output",
		Args: func(_ *cobra.Command, args []string) error {
			if input != "" && len(args) > 0 {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("use either --input or one positional input, not both"))
			}
			if input == "" && len(args) != 1 {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--input or one positional input is required"))
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			inputPath := input
			if inputPath == "" {
				inputPath = args[0]
			}
			if target == "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--to is required"))
			}
			if destination == "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--output is required"))
			}
			if filepath.Clean(inputPath) == filepath.Clean(destination) {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("output path is the input path"))
			}
			format, err := dicom.ResolveTransferSyntax(target)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			inputInfo, err := os.Stat(inputPath)
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if inputInfo.IsDir() {
				condition, err := loadTranscodeFilter(runtime, root.rulesPath, filterName)
				if err != nil {
					return apperr.Wrap(apperr.KindInput, err)
				}
				entries, err := files.Scan(inputPath, recursive, func(path string) (bool, string, error) {
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
					outputPath, pathErr := files.OutputPath(entry.Path, inputPath, destination, !flatten)
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
			if err := transcodeFile(inputPath, destination, format.Syntax); err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			return nil
		},
	}
	command.Flags().StringVarP(&input, "input", "i", "", root.localizer.FlagUsage("input", "input DICOM file or directory"))
	command.Flags().StringVar(&target, "to", "", root.localizer.FlagUsage("to", "target transfer syntax name, short name, or UID, for example JPEG 2000 Lossless, jpeg2000-lossless, or 1.2.840.10008.1.2.4.90"))
	command.Flags().StringVarP(&destination, "output", "o", "", root.localizer.FlagUsage("output", "output DICOM file or directory"))
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, root.localizer.FlagUsage("recursive", "scan subdirectories"))
	command.Flags().BoolVar(&failFast, "fail-fast", false, root.localizer.FlagUsage("fail-fast", "stop after the first file failure"))
	command.Flags().BoolVar(&flatten, "flatten", false, root.localizer.FlagUsage("flatten", "do not preserve input directory structure"))
	command.Flags().StringVar(&filterName, "filter", "", root.localizer.FlagUsage("filter", "named rules filter for directory input"))
	var asJSON bool
	formatsText := root.localizer.Command("transcode formats")
	formats := &cobra.Command{
		Use:   "formats",
		Short: formatsText.Short,
		Long:  formatsText.Long,
		Example: "  dicom-cli transcode formats\n" +
			"  dicom-cli transcode formats --json",
		Args: noArgs,
		RunE: func(*cobra.Command, []string) error {
			available := dicom.RuntimeCodecs()
			if asJSON {
				return json.NewEncoder(runtime.Stdout).Encode(available)
			}
			header := "NAME\t--TO SHORT NAME\tUID\tENCODE\tDECODE\tSTATUS"
			if root.localizer.IsChineseSimplified() {
				header = "标准名称\t--TO 短名称\tUID\t编码\t解码\t状态"
			}
			if _, err := fmt.Fprintln(runtime.Stdout, header); err != nil {
				return err
			}
			for _, format := range available {
				marker := ""
				if format.Experimental {
					marker = " experimental"
				}
				if _, err := fmt.Fprintf(runtime.Stdout, "%s\t%s\t%s\tencode=%t\tdecode=%t%s\n", format.Name, format.Alias, format.UID, format.Encode, format.Decode, marker); err != nil {
					return err
				}
			}
			return nil
		},
	}
	formats.Flags().BoolVarP(&asJSON, "json", "j", false, root.localizer.FlagUsage("json", "write JSON output"))
	localizedHelpFlag(command, root.localizer)
	localizedHelpFlag(formats, root.localizer)
	command.AddCommand(formats)
	return command
}

func transcodeHelpText(base string, chinese bool) string {
	header := "Available --to values in this binary (standard name | --to short name | UID):"
	experimental := " experimental"
	if chinese {
		header = "当前二进制可用于 --to 的值（标准名称 | --to 短名称 | UID）："
		experimental = " experimental"
	}
	lines := make([]string, 0, len(dicom.RuntimeCodecs())+2)
	lines = append(lines, base, header)
	for _, format := range dicom.RuntimeCodecs() {
		marker := ""
		if format.Experimental {
			marker = experimental
		}
		lines = append(lines, fmt.Sprintf("  %s | %s | %s%s", format.Name, format.Alias, format.UID, marker))
	}
	return strings.Join(lines, "\n")
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
