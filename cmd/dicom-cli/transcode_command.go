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
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

func newTranscodeCommand(runtime Runtime, _ *rootOptions) *cobra.Command {
	var target, destination string
	command := &cobra.Command{
		Use:   "transcode <file>",
		Short: "Re-encode DICOM transfer syntaxes",
		Args:  cobra.ExactArgs(1),
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
			if _, err := os.Stat(destination); err == nil {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("output file %q already exists", destination))
			} else if !os.IsNotExist(err) {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			format, err := dicom.ResolveTransferSyntax(target)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			parsed, err := parser.ParseFile(args[0])
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			converted, err := convertpkg.Transcode(parsed.Dataset, parsed.TransferSyntax, format.Syntax)
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if err := writer.WriteFile(destination, converted, writer.WithTransferSyntax(format.Syntax)); err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&target, "to", "", "target transfer syntax alias or UID")
	command.Flags().StringVarP(&destination, "output", "o", "", "output DICOM file")
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
