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
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
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
	recursive        bool
	failFast         bool
	flatten          bool
}

func newConvertCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	options := dicomExportOptions{format: "png"}
	command := &cobra.Command{
		Use:   "convert",
		Short: "Export DICOM images and metadata",
		Long:  "DICOM input is exported either as rendered image frames or as metadata JSON. Select the image or json subcommand; conversion never rewrites the source DICOM file.",
		Args:  noArgs,
	}

	imageCommand := &cobra.Command{
		Use:   "image <input>",
		Short: "Export DICOM pixel data as PNG or JPEG",
		Long:  "Export DICOM pixel data as PNG or JPEG. Frame numbers start at 1; without --frame or --all-frames, the first frame is exported. Binary stdout requires exactly one result.",
		Example: "  dicom-cli convert image --format png --output image.png image.dcm\n" +
			"  dicom-cli convert image --all-frames --output frames image.dcm",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDICOMExport(runtime, args[0], options)
		},
	}
	imageCommand.Flags().StringVar(&options.format, "format", "png", "output format: png or jpeg")
	imageCommand.Flags().StringVarP(&options.destination, "output", "o", "", "output file, directory, or -")
	imageCommand.Flags().IntVar(&options.frame, "frame", 0, "one-based frame number")
	imageCommand.Flags().BoolVar(&options.allFrames, "all-frames", false, "export every image frame")
	imageCommand.Flags().BoolVarP(&options.recursive, "recursive", "r", false, "scan subdirectories")
	imageCommand.Flags().BoolVar(&options.failFast, "fail-fast", false, "stop after the first file failure")
	imageCommand.Flags().BoolVar(&options.flatten, "flatten", false, "do not preserve input directory structure")

	jsonCommand := &cobra.Command{
		Use:     "json <input>",
		Short:   "Export DICOM metadata as JSON",
		Long:    "Export DICOM metadata as JSON. PixelData is summarized by default; use --include-pixel-data only when the full pixel bytes are required.",
		Example: "  dicom-cli convert json --output metadata.json image.dcm",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			options.format = "json"
			return runDICOMExport(runtime, args[0], options)
		},
	}
	jsonCommand.Flags().StringVarP(&options.destination, "output", "o", "", "output file or -")
	jsonCommand.Flags().BoolVar(&options.includePixelData, "include-pixel-data", false, "include PixelData bytes")
	jsonCommand.Flags().BoolVarP(&options.recursive, "recursive", "r", false, "scan subdirectories")
	jsonCommand.Flags().BoolVar(&options.failFast, "fail-fast", false, "stop after the first file failure")
	jsonCommand.Flags().BoolVar(&options.flatten, "flatten", false, "do not preserve input directory structure")

	command.AddCommand(imageCommand, jsonCommand)
	return command
}

func runImageToDICOMWithMetadata(runtime Runtime, root *rootOptions, input, patientName, templateName, referencePath, destination string, recursive, failFast, flatten bool) error {
	template, reference, err := imageMetadataSources(runtime, root.rulesPath, templateName, referencePath)
	if err != nil {
		return apperr.Wrap(apperr.KindInput, err)
	}
	return runImageToDICOM(runtime, input, patientName, destination, recursive, failFast, flatten, template, reference)
}

func runImageToDICOM(runtime Runtime, input, patientName, destination string, recursive, failFast, flatten bool, template map[string]string, reference *dataset.Dataset) error {
	if patientName == "" {
		if reference != nil {
			patientName, _ = reference.GetString(tag.PatientName)
		}
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
				generated := dataset.Clone()
				buildErr = convertpkg.ApplyMetadata(dataset, template)
				if buildErr == nil && reference != nil {
					dataset.Merge(reference, true)
				}
				if buildErr == nil {
					dataset.Merge(generated, true)
					buildErr = convertpkg.ApplyMetadata(dataset, map[string]string{"PatientName": patientName})
				}
			}
			if buildErr != nil {
				err = buildErr
			} else {
				outputPath := destination
				if info.IsDir() {
					outputPath, err = files.OutputPath(entry.Path, input, destination, !flatten)
					if err == nil {
						outputPath, err = convertedDICOMOutputPath(outputPath)
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
		return apperr.Wrap(apperr.KindOperation, fmt.Errorf("encapsulate image failed for %d file(s)", failed))
	}
	return nil
}

func convertedDICOMOutputPath(path string) (string, error) {
	path = strings.TrimSuffix(path, filepath.Ext(path)) + ".dcm"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, nil
	} else if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(path, ".dcm")
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s-%d.dcm", base, index)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

func imageMetadataSources(runtime Runtime, configuredRules, templateName, referencePath string) (map[string]string, *dataset.Dataset, error) {
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
	var reference *dataset.Dataset
	if referencePath != "" {
		parsed, err := parser.ParseFile(referencePath)
		if err != nil {
			return nil, nil, err
		}
		reference = parsed.Dataset.Clone()
	}
	return template, reference, nil
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
	info, err := os.Stat(input)
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	if info.IsDir() {
		return runDICOMExportDirectory(runtime, input, format, options)
	}
	return runDICOMExportFile(runtime, input, format, options, options.destination)
}

func runDICOMExportDirectory(runtime Runtime, input, format string, options dicomExportOptions) error {
	if options.destination == "-" {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("binary stdout requires exactly one input file"))
	}
	destination := options.destination
	if destination == "" {
		workingDirectory, err := runtime.Getwd()
		if err != nil {
			return err
		}
		destination = files.DefaultOutputDirectory(workingDirectory, "convert")
	}
	entries, err := files.Scan(input, options.recursive, func(path string) (bool, string, error) {
		if strings.EqualFold(filepath.Ext(path), ".dcm") {
			return true, "", nil
		}
		return false, "unsupported DICOM extension", nil
	})
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	failed := 0
	for _, entry := range entries {
		if entry.Skipped {
			continue
		}
		if err := runDICOMExportFile(runtime, entry.Path, format, options, destinationForDICOMExport(entry.Path, input, destination, format, options)); err != nil {
			failed++
			if options.failFast {
				break
			}
		}
	}
	if failed > 0 {
		return apperr.Wrap(apperr.KindOperation, fmt.Errorf("convert export failed for %d file(s)", failed))
	}
	return nil
}

func destinationForDICOMExport(path, root, destination, format string, options dicomExportOptions) string {
	preserve := !options.flatten
	name, err := files.OutputPath(path, root, destination, preserve)
	if err != nil {
		return ""
	}
	if options.allFrames {
		return filepath.Dir(name)
	}
	return strings.TrimSuffix(name, filepath.Ext(name)) + "." + format
}

func runDICOMExportFile(runtime Runtime, input, format string, options dicomExportOptions, destination string) error {
	parsed, err := parser.ParseFile(input)
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	if format == "json" {
		content, err := convertpkg.ExportJSON(parsed.Dataset, options.includePixelData)
		if err != nil {
			return apperr.Wrap(apperr.KindOperation, err)
		}
		return writeExport(runtime, input, destination, "json", content)
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
	if len(frames) > 1 && destination == "" {
		workingDirectory, err := runtime.Getwd()
		if err != nil {
			return err
		}
		destination = files.DefaultOutputDirectory(workingDirectory, "convert")
	}
	if len(frames) > 1 && destination != "" {
		if err := os.MkdirAll(destination, 0o755); err != nil {
			return apperr.Wrap(apperr.KindOperation, err)
		}
	}
	for _, frame := range frames {
		content, err := convertpkg.ExportFrame(parsed.Dataset, parsed.TransferSyntax, frame, convertpkg.ImageFormat(format))
		if err != nil {
			return apperr.Wrap(apperr.KindOperation, err)
		}
		frameDestination := destination
		if len(frames) > 1 {
			frameDestination = filepath.Join(destination, frameOutputName(input, format, frame))
		}
		if err := writeExport(runtime, input, frameDestination, format, content); err != nil {
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
