package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	convertpkg "github.com/cocosip/dicom-cli/internal/convert"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/uid"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

type dicomExportOptions struct {
	format           string
	destination      string
	frame            int
	allFrames        bool
	includePixelData bool
}

func newConvertCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	options := dicomExportOptions{format: "png"}
	var patientName, templateName, referencePath string
	var recursive, failFast, flatten bool
	command := &cobra.Command{
		Use:   "convert <input>",
		Short: "Convert DICOM images and metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if strings.EqualFold(options.format, "dicom") {
				return runImageToDICOMWithMetadata(runtime, root, args[0], patientName, templateName, referencePath, options.destination, recursive, failFast, flatten)
			}
			return runDICOMExport(runtime, args[0], options)
		},
	}
	command.Flags().StringVar(&options.format, "to", "", "output format: png, jpeg, json, or dicom")
	command.Flags().StringVarP(&options.destination, "output", "o", "", "output file, directory, or -")
	command.Flags().IntVar(&options.frame, "frame", 0, "one-based frame number")
	command.Flags().BoolVar(&options.allFrames, "all-frames", false, "export every image frame")
	command.Flags().BoolVar(&options.includePixelData, "include-pixel-data", false, "include PixelData bytes in JSON")
	command.Flags().StringVar(&patientName, "patient-name", "", "required PatientName for DICOM output")
	command.Flags().StringVar(&templateName, "template", "", "named DICOM template from rules")
	command.Flags().StringVar(&referencePath, "reference", "", "reference DICOM metadata source")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "scan subdirectories for DICOM output")
	command.Flags().BoolVar(&failFast, "fail-fast", false, "stop after the first DICOM output failure")
	command.Flags().BoolVar(&flatten, "flatten", false, "do not preserve input directory structure for DICOM output")

	imageCommand := &cobra.Command{
		Use:   "image <input>",
		Short: "Export DICOM pixel data as PNG or JPEG",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDICOMExport(runtime, args[0], options)
		},
	}
	imageCommand.Flags().StringVar(&options.format, "format", "png", "output format: png or jpeg")
	imageCommand.Flags().StringVarP(&options.destination, "output", "o", "", "output file, directory, or -")
	imageCommand.Flags().IntVar(&options.frame, "frame", 0, "one-based frame number")
	imageCommand.Flags().BoolVar(&options.allFrames, "all-frames", false, "export every image frame")

	jsonCommand := &cobra.Command{
		Use:   "json <input>",
		Short: "Export DICOM metadata as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			options.format = "json"
			return runDICOMExport(runtime, args[0], options)
		},
	}
	jsonCommand.Flags().StringVarP(&options.destination, "output", "o", "", "output file or -")
	jsonCommand.Flags().BoolVar(&options.includePixelData, "include-pixel-data", false, "include PixelData bytes")

	var destination string
	dicomCommand := &cobra.Command{
		Use:   "dicom <input>",
		Short: "Encapsulate PNG or JPEG images as Secondary Capture DICOM",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runImageToDICOMWithMetadata(runtime, root, args[0], patientName, templateName, referencePath, destination, recursive, failFast, flatten)
		},
	}
	dicomCommand.Flags().StringVar(&patientName, "patient-name", "", "required PatientName for created DICOM files")
	dicomCommand.Flags().StringVar(&templateName, "template", "", "named DICOM template from rules")
	dicomCommand.Flags().StringVar(&referencePath, "reference", "", "reference DICOM metadata source")
	dicomCommand.Flags().StringVarP(&destination, "output", "o", "", "DICOM output file or directory")
	dicomCommand.Flags().BoolVarP(&recursive, "recursive", "r", false, "scan subdirectories")
	dicomCommand.Flags().BoolVar(&failFast, "fail-fast", false, "stop after the first file failure")
	dicomCommand.Flags().BoolVar(&flatten, "flatten", false, "do not preserve input directory structure")

	command.AddCommand(imageCommand, jsonCommand, dicomCommand)
	return command
}

func runImageToDICOMWithMetadata(runtime Runtime, root *rootOptions, input, patientName, templateName, referencePath, destination string, recursive, failFast, flatten bool) error {
	template, reference, err := imageMetadataSources(runtime, root.rulesPath, templateName, referencePath)
	if err != nil {
		return apperr.Wrap(apperr.KindInput, err)
	}
	return runImageToDICOM(runtime, input, patientName, destination, recursive, failFast, flatten, template, reference)
}

func runImageToDICOM(runtime Runtime, input, patientName, destination string, recursive, failFast, flatten bool, template, reference map[string]string) error {
	if patientName == "" {
		patientName = reference["PatientName"]
		if patientName == "" {
			patientName = template["PatientName"]
		}
		if patientName == "" {
			return apperr.Wrap(apperr.KindInput, fmt.Errorf("--patient-name or metadata PatientName is required"))
		}
	}
	info, err := os.Stat(input)
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	if destination == "" {
		workingDirectory, err := runtime.Getwd()
		if err != nil {
			return err
		}
		destination = files.DefaultOutputDirectory(workingDirectory, "convert")
	}
	if !info.IsDir() && destination != "" && filepath.Ext(destination) == "" {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("single image output must be a .dcm file"))
	}
	entries, err := files.Scan(input, recursive, func(path string) (bool, string, error) {
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
			return false, "unsupported image extension", nil
		}
		return true, "", nil
	})
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	studyUID := uid.GenerateDerivedFromUUID().UID()
	seriesUID := uid.GenerateDerivedFromUUID().UID()
	failed := 0
	for _, entry := range entries {
		if entry.Skipped {
			continue
		}
		imageValue, err := convertpkg.LoadImage(entry.Path)
		if err == nil {
			dataset, buildErr := convertpkg.NewSecondaryCapture(imageValue, convertpkg.SecondaryCaptureOptions{PatientName: patientName, StudyUID: studyUID, SeriesUID: seriesUID})
			if buildErr == nil {
				buildErr = convertpkg.ApplyMetadata(dataset, template, reference, map[string]string{"PatientName": patientName})
			}
			if buildErr != nil {
				err = buildErr
			} else {
				outputPath := destination
				if info.IsDir() {
					outputPath, err = files.OutputPath(entry.Path, input, destination, !flatten)
					if err == nil {
						outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dcm"
					}
				}
				if err == nil {
					if err = os.MkdirAll(filepath.Dir(outputPath), 0o755); err == nil {
						err = writer.WriteFile(outputPath, dataset, writer.WithTransferSyntax(transfer.ExplicitVRLittleEndian))
					}
				}
			}
		}
		if err != nil {
			failed++
			if failFast {
				break
			}
		}
	}
	if failed > 0 {
		return apperr.Wrap(apperr.KindOperation, fmt.Errorf("convert dicom failed for %d file(s)", failed))
	}
	return nil
}

func imageMetadataSources(runtime Runtime, configuredRules, templateName, referencePath string) (map[string]string, map[string]string, error) {
	template := map[string]string{}
	if templateName != "" {
		path, err := rulesPath(runtime, configuredRules, nil)
		if err != nil {
			return nil, nil, err
		}
		file, err := rules.Load(path)
		if err != nil {
			return nil, nil, err
		}
		selected, ok := file.DICOMTemplates[templateName]
		if !ok {
			return nil, nil, fmt.Errorf("DICOM template %q does not exist", templateName)
		}
		template = selected.Tags
	}
	reference := map[string]string{}
	if referencePath != "" {
		parsed, err := parser.ParseFile(referencePath)
		if err != nil {
			return nil, nil, err
		}
		for _, path := range []string{"PatientName", "PatientID", "StudyInstanceUID", "SeriesInstanceUID"} {
			if value, ok := parsed.Dataset.GetString(elementTag(path)); ok && value != "" {
				reference[path] = value
			}
		}
	}
	return template, reference, nil
}

func elementTag(keyword string) *tag.Tag {
	parsed, err := tag.ParseKeyword(keyword)
	if err != nil {
		return nil
	}
	return &parsed
}

func runDICOMExport(runtime Runtime, input string, options dicomExportOptions) error {
	format := strings.ToLower(options.format)
	if format == "jpg" {
		format = string(convertpkg.JPEG)
	}
	if format != string(convertpkg.PNG) && format != string(convertpkg.JPEG) && format != "json" {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("unsupported conversion format %q", options.format))
	}
	if options.frame < 0 {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("--frame must be positive"))
	}
	if options.allFrames && format == "json" {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("--all-frames applies only to image output"))
	}
	if options.allFrames && options.destination == "-" {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("binary stdout requires exactly one result"))
	}
	if info, err := os.Stat(input); err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	} else if info.IsDir() {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("convert export requires a DICOM file"))
	}
	parsed, err := parser.ParseFile(input)
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	if format == "json" {
		content, err := convertpkg.ExportJSON(parsed.Dataset, options.includePixelData)
		if err != nil {
			return apperr.Wrap(apperr.KindOperation, err)
		}
		return writeExport(runtime, input, options.destination, "json", content)
	}
	frameCount, err := convertpkg.FrameCount(parsed.Dataset)
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	frames := []int{0}
	if options.frame > 0 {
		frames = []int{options.frame - 1}
	}
	if options.allFrames {
		frames = make([]int, frameCount)
		for index := range frames {
			frames[index] = index
		}
	}
	if len(frames) > 1 && options.destination != "" {
		if err := os.MkdirAll(options.destination, 0o755); err != nil {
			return apperr.Wrap(apperr.KindOperation, err)
		}
	}
	for _, frame := range frames {
		content, err := convertpkg.ExportFrame(parsed.Dataset, parsed.TransferSyntax, frame, convertpkg.ImageFormat(format))
		if err != nil {
			return apperr.Wrap(apperr.KindOperation, err)
		}
		destination := options.destination
		if len(frames) > 1 {
			destination = filepath.Join(options.destination, frameOutputName(input, format, frame))
		}
		if err := writeExport(runtime, input, destination, format, content); err != nil {
			return err
		}
	}
	return nil
}

func writeExport(runtime Runtime, input, destination, extension string, content []byte) error {
	if destination == "-" {
		_, err := runtime.Stdout.Write(content)
		return err
	}
	if destination == "" {
		workingDirectory, err := runtime.Getwd()
		if err != nil {
			return err
		}
		destination = filepath.Join(workingDirectory, "convert", strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))+"."+extension)
	}
	inputPath, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	outputPath, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if filepath.Clean(inputPath) == filepath.Clean(outputPath) {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("output path is the input path"))
	}
	if _, err := os.Stat(destination); err == nil {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("output file %q already exists", destination))
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0o600)
}

func frameOutputName(input, extension string, frame int) string {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	return fmt.Sprintf("%s-frame-%04d.%s", base, frame+1, extension)
}
