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
		"[File]",
		fmt.Sprintf("  Path: %s", report.File.Path),
		fmt.Sprintf("  Transfer Syntax: %s", report.File.TransferSyntax),
		"",
		"[Patient]",
		fmt.Sprintf("  Name: %s", report.Patient.Name),
		fmt.Sprintf("  ID: %s", report.Patient.ID),
		fmt.Sprintf("  Birth Date: %s", report.Patient.BirthDate),
		fmt.Sprintf("  Sex: %s", report.Patient.Sex),
		"",
		"[Study]",
		fmt.Sprintf("  Instance UID: %s", report.Study.InstanceUID),
		fmt.Sprintf("  Modality: %s", report.Study.Modality),
		fmt.Sprintf("  Date: %s", report.Study.Date),
		fmt.Sprintf("  Time: %s", report.Study.Time),
		fmt.Sprintf("  Accession Number: %s", report.Study.AccessionNumber),
		fmt.Sprintf("  Description: %s", report.Study.Description),
		fmt.Sprintf("  Referring Physician: %s", report.Study.ReferringPhysician),
		"",
		"[Series]",
		fmt.Sprintf("  Instance UID: %s", report.Series.InstanceUID),
		fmt.Sprintf("  Number: %s", report.Series.Number),
		fmt.Sprintf("  Description: %s", report.Series.Description),
		fmt.Sprintf("  Body Part: %s", report.Series.BodyPart),
		fmt.Sprintf("  Laterality: %s", report.Series.Laterality),
		fmt.Sprintf("  Protocol: %s", report.Series.ProtocolName),
		"",
		"[Instance]",
		fmt.Sprintf("  SOP Class UID: %s", report.Instance.SOPClassUID),
		fmt.Sprintf("  SOP Instance UID: %s", report.Instance.SOPInstanceUID),
		fmt.Sprintf("  Number: %s", report.Instance.Number),
		fmt.Sprintf("  Image Position: %s", report.Instance.ImagePosition),
		fmt.Sprintf("  Image Orientation: %s", report.Instance.ImageOrientation),
		fmt.Sprintf("  Slice Thickness: %s", report.Instance.SliceThickness),
		fmt.Sprintf("  Spacing Between Slices: %s", report.Instance.SpacingBetweenSlices),
		"",
		"[Pixel]",
		fmt.Sprintf("  Rows: %d", report.Pixel.Rows),
		fmt.Sprintf("  Columns: %d", report.Pixel.Columns),
		fmt.Sprintf("  Frames: %d", report.Pixel.Frames),
		fmt.Sprintf("  Bytes: %d", report.Pixel.Bytes),
		fmt.Sprintf("  Samples Per Pixel: %d", report.Pixel.SamplesPerPixel),
		fmt.Sprintf("  Photometric Interpretation: %s", report.Pixel.PhotometricInterpretation),
		fmt.Sprintf("  Bits Allocated: %d", report.Pixel.BitsAllocated),
		fmt.Sprintf("  Bits Stored: %d", report.Pixel.BitsStored),
		fmt.Sprintf("  High Bit: %d", report.Pixel.HighBit),
		fmt.Sprintf("  Pixel Representation: %d", report.Pixel.PixelRepresentation),
		fmt.Sprintf("  Pixel Spacing: %s", report.Pixel.PixelSpacing),
		fmt.Sprintf("  Window Center: %s", report.Pixel.WindowCenter),
		fmt.Sprintf("  Window Width: %s", report.Pixel.WindowWidth),
	}
	if len(report.Elements) > 0 {
		lines = append(lines, "", "[Elements]")
		for _, elem := range report.Elements {
			lines = append(lines, fmt.Sprintf("%s %s %s = %s", elem.Tag, elem.VR, elem.Path, elem.Value))
		}
	}
	if len(report.Tags) > 0 {
		lines = append(lines, "", "[Selected Tags]")
	}
	for _, elem := range report.Tags {
		lines = append(lines, fmt.Sprintf("%s %s %s = %s", elem.Tag, elem.VR, elem.Path, elem.Value))
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}
