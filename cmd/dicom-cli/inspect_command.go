package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/inspect"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

func newInspectCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	var all bool
	var tags []string
	var profile string
	var asJSON bool
	var destination string
	command := &cobra.Command{
		Use: "inspect <file>", Short: "Inspect a single DICOM file", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := requireRegularFile(args[0]); err != nil {
				return err
			}
			if profile != "" {
				path, err := rulesPath(runtime, root.rulesPath, nil)
				if err != nil {
					return apperr.Wrap(apperr.KindInput, err)
				}
				file, err := rules.Load(path)
				if err != nil {
					return apperr.Wrap(apperr.KindInput, err)
				}
				selected, ok := file.Inspect.Profiles[profile]
				if !ok {
					return apperr.Wrap(apperr.KindInput, fmt.Errorf("inspect profile %q does not exist", profile))
				}
				tags = append(tags, selected.Tags...)
			}
			parsed, err := parser.ParseFile(args[0])
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			report, err := inspect.Build(args[0], parsed, inspect.Options{All: all, Tags: tags})
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			content, err := renderInspect(report, asJSON)
			if err != nil {
				return err
			}
			return writeReport(runtime, args[0], destination, content)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "include every data element")
	command.Flags().StringArrayVar(&tags, "tag", nil, "DICOM keyword or hexadecimal Tag path")
	command.Flags().StringVarP(&profile, "profile", "p", "", "inspect profile from rules")
	command.Flags().BoolVarP(&asJSON, "json", "j", false, "write JSON")
	command.Flags().StringVarP(&destination, "output", "o", "", "report output path")
	return command
}

func renderInspect(report inspect.Report, asJSON bool) ([]byte, error) {
	if asJSON {
		return json.MarshalIndent(report, "", "  ")
	}
	lines := []string{
		fmt.Sprintf("File: %s", report.File.Path), fmt.Sprintf("Transfer Syntax: %s", report.File.TransferSyntax),
		fmt.Sprintf("Patient: %s (%s)", report.Patient.Name, report.Patient.ID), fmt.Sprintf("Study: %s (%s)", report.Study.InstanceUID, report.Study.Modality),
		fmt.Sprintf("Series: %s", report.Series.InstanceUID), fmt.Sprintf("Pixel: %dx%d, frames=%d, bytes=%d", report.Pixel.Rows, report.Pixel.Columns, report.Pixel.Frames, report.Pixel.Bytes),
	}
	for _, elem := range append(report.Elements, report.Tags...) {
		lines = append(lines, fmt.Sprintf("%s %s %s = %s", elem.Tag, elem.VR, elem.Path, elem.Value))
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}
