package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/i18n"
	"github.com/cocosip/dicom-cli/internal/inspect"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

func newInspectCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	localizer := root.localizer
	text := localizer.Command("inspect")
	var all bool
	var tags []string
	var profile string
	var asJSON bool
	var destination string
	command := &cobra.Command{
		Use:   "inspect <file>",
		Short: text.Short,
		Long:  text.Long,
		Example: "  dicom-cli inspect image.dcm\n" +
			"  dicom-cli inspect --tag PatientName --tag 0040,A730[0].0040,A160 image.dcm",
		Args: cobra.ExactArgs(1),
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
			content, err := renderInspect(report, asJSON, localizer)
			if err != nil {
				return err
			}
			return writeReport(runtime, args[0], destination, content)
		},
	}
	localizedHelpFlag(command, localizer)
	command.Flags().BoolVar(&all, "all", false, localizer.Text(i18n.FlagInspectAll))
	command.Flags().StringArrayVar(&tags, "tag", nil, localizer.Text(i18n.FlagInspectTag))
	command.Flags().StringVarP(&profile, "profile", "p", "", localizer.Text(i18n.FlagInspectProfile))
	command.Flags().BoolVarP(&asJSON, "json", "j", false, localizer.Text(i18n.FlagJSON))
	command.Flags().StringVarP(&destination, "output", "o", "", localizer.Text(i18n.FlagReportOutput))
	return command
}

func renderInspect(report inspect.Report, asJSON bool, localizer i18n.Localizer) ([]byte, error) {
	if asJSON {
		return json.MarshalIndent(report, "", "  ")
	}
	lines := []string{
		"[File]",
		fmt.Sprintf("  Path: %s", report.File.Path),
		fmt.Sprintf("  Transfer Syntax: %s", report.File.TransferSyntax),
		"",
		"[Encoding]",
		fmt.Sprintf("  Transfer Syntax UID: %s", report.Encoding.UID),
		fmt.Sprintf("  Transfer Syntax Name: %s", report.Encoding.Name),
		fmt.Sprintf("  VR Encoding: %s", report.Encoding.VREncoding),
		fmt.Sprintf("  Byte Order: %s", report.Encoding.ByteOrder),
		fmt.Sprintf("  Encapsulated: %t", report.Encoding.Encapsulated),
		fmt.Sprintf("  Lossy: %t", report.Encoding.Lossy),
		fmt.Sprintf("  Lossy Compression Method: %s", report.Encoding.LossyCompressionMethod),
		fmt.Sprintf("  Deflated: %t", report.Encoding.Deflated),
		fmt.Sprintf("  Retired: %t", report.Encoding.Retired),
		"",
		"[File Meta]",
		fmt.Sprintf("  Media Storage SOP Class UID: %s", report.FileMeta.MediaStorageSOPClassUID),
		fmt.Sprintf("  Media Storage SOP Instance UID: %s", report.FileMeta.MediaStorageSOPInstanceUID),
		fmt.Sprintf("  Implementation Class UID: %s", report.FileMeta.ImplementationClassUID),
		fmt.Sprintf("  Implementation Version Name: %s", report.FileMeta.ImplementationVersionName),
		fmt.Sprintf("  Source Application AE Title: %s", report.FileMeta.SourceApplicationAETitle),
		"",
		"[Equipment]",
		fmt.Sprintf("  Specific Character Set: %s", report.Equipment.SpecificCharacterSet),
		fmt.Sprintf("  Manufacturer: %s", report.Equipment.Manufacturer),
		fmt.Sprintf("  Model: %s", report.Equipment.Model),
		fmt.Sprintf("  Station: %s", report.Equipment.Station),
		fmt.Sprintf("  Software Versions: %s", report.Equipment.SoftwareVersions),
		"",
		"[Patient]",
		fmt.Sprintf("  Name: %s", report.Patient.Name),
		fmt.Sprintf("  ID: %s", report.Patient.ID),
		fmt.Sprintf("  Birth Date: %s", report.Patient.BirthDate),
		fmt.Sprintf("  Sex: %s", report.Patient.Sex),
		"",
		"[Study]",
		fmt.Sprintf("  Instance UID: %s", report.Study.InstanceUID),
		fmt.Sprintf("  Study ID: %s", report.Study.ID),
		fmt.Sprintf("  Modality: %s", report.Study.Modality),
		fmt.Sprintf("  Date: %s", report.Study.Date),
		fmt.Sprintf("  Time: %s", report.Study.Time),
		fmt.Sprintf("  Accession Number: %s", report.Study.AccessionNumber),
		fmt.Sprintf("  Description: %s", report.Study.Description),
		fmt.Sprintf("  Referring Physician: %s", report.Study.ReferringPhysician),
		fmt.Sprintf("  Institution: %s", report.Study.Institution),
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
		fmt.Sprintf("  Image Type: %s", report.Instance.ImageType),
		fmt.Sprintf("  Content Date: %s", report.Instance.ContentDate),
		fmt.Sprintf("  Content Time: %s", report.Instance.ContentTime),
		fmt.Sprintf("  Acquisition Number: %s", report.Instance.AcquisitionNumber),
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
		fmt.Sprintf("  Planar Configuration: %d", report.Pixel.PlanarConfiguration),
		fmt.Sprintf("  Pixel Aspect Ratio: %s", report.Pixel.PixelAspectRatio),
		fmt.Sprintf("  Rescale Intercept: %s", report.Pixel.RescaleIntercept),
		fmt.Sprintf("  Rescale Slope: %s", report.Pixel.RescaleSlope),
		fmt.Sprintf("  Rescale Type: %s", report.Pixel.RescaleType),
		fmt.Sprintf("  VOI LUT Function: %s", report.Pixel.VOILUTFunction),
		fmt.Sprintf("  Lossy Image Compression: %s", report.Pixel.LossyImageCompression),
		fmt.Sprintf("  Lossy Image Compression Ratio: %s", report.Pixel.LossyImageCompressionRatio),
		fmt.Sprintf("  Lossy Image Compression Method: %s", report.Pixel.LossyImageCompressionMethod),
	}
	var fileMetaElements, datasetElements []inspect.ElementReport
	for _, elem := range report.Elements {
		if elem.Source == "file_meta" {
			fileMetaElements = append(fileMetaElements, elem)
		} else {
			datasetElements = append(datasetElements, elem)
		}
	}
	if len(fileMetaElements) > 0 {
		lines = append(lines, "", "[File Meta Elements]")
		for _, elem := range fileMetaElements {
			lines = append(lines, fmt.Sprintf("%s %s %s = %s", elem.Tag, elem.VR, elem.Path, elem.Value))
		}
	}
	if len(datasetElements) > 0 {
		lines = append(lines, "", "[Elements]")
		for _, elem := range datasetElements {
			lines = append(lines, fmt.Sprintf("%s %s %s = %s", elem.Tag, elem.VR, elem.Path, elem.Value))
		}
	}
	if len(report.Tags) > 0 {
		lines = append(lines, "", "[Selected Tags]")
	}
	for _, elem := range report.Tags {
		lines = append(lines, fmt.Sprintf("%s %s %s = %s", elem.Tag, elem.VR, elem.Path, elem.Value))
	}
	return []byte(localizer.ReplaceInspectLabels(strings.Join(lines, "\n") + "\n")), nil
}
