package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/app"
	"github.com/cocosip/dicom-cli/internal/apperr"
	convertpkg "github.com/cocosip/dicom-cli/internal/convert"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/dicom-cli/internal/output"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/uid"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

type dicomExportOptions struct {
	format      string
	destination string
	frame       int
	allFrames   bool
	recursive   bool
	failFast    bool
	flatten     bool
}

func newConvertCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("convert")
	options := dicomExportOptions{format: "png"}
	var imageInput, metadataInput, metadataFormat string
	command := &cobra.Command{
		Use:   "convert",
		Short: text.Short,
		Long:  text.Long,
		Args:  noArgs,
	}

	addExportFlags := func(command *cobra.Command, input *string, outputHelp string) {
		command.Flags().StringVarP(input, "input", "i", "", root.localizer.FlagUsage("input", "input DICOM file or directory"))
		command.Flags().StringVarP(&options.destination, "output", "o", "", root.localizer.FlagUsage("output", outputHelp))
		command.Flags().BoolVarP(&options.recursive, "recursive", "r", false, root.localizer.FlagUsage("recursive", "scan subdirectories"))
		command.Flags().BoolVar(&options.failFast, "fail-fast", false, root.localizer.FlagUsage("fail-fast", "stop after the first file failure"))
		command.Flags().BoolVar(&options.flatten, "flatten", false, root.localizer.FlagUsage("flatten", "do not preserve input directory structure"))
	}

	imageText := root.localizer.Command("convert image")
	imageCommand := &cobra.Command{
		Use:   "image --input <file-or-directory>",
		Short: imageText.Short,
		Long:  imageText.Long,
		Example: "  dicom-cli convert image --input image.dcm --format png --output image.png\n" +
			"  dicom-cli convert image --input image.dcm --all-frames --output frames",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if imageInput == "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--input is required"))
			}
			return runDICOMExport(command.Context(), runtime, imageInput, options)
		},
	}
	addExportFlags(imageCommand, &imageInput, "output file, directory, or -")
	imageCommand.Flags().StringVar(&options.format, "format", "png", root.localizer.FlagUsage("format", "output format: png or jpeg"))
	imageCommand.Flags().IntVar(&options.frame, "frame", 0, root.localizer.FlagUsage("frame", "one-based frame number"))
	imageCommand.Flags().BoolVar(&options.allFrames, "all-frames", false, root.localizer.FlagUsage("all-frames", "export every image frame"))

	metadataText := root.localizer.Command("convert metadata")
	metadataCommand := &cobra.Command{
		Use:   "metadata --input <file-or-directory>",
		Short: metadataText.Short,
		Long:  metadataText.Long,
		Example: "  dicom-cli convert metadata --input image.dcm --format json --output metadata.json\n" +
			"  dicom-cli convert metadata --input image.dcm --format xml --output metadata.xml\n" +
			"  dicom-cli convert metadata --input image.dcm --format pixeldata --output pixels.bin",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if metadataInput == "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--input is required"))
			}
			options.format = metadataFormat
			return runDICOMExport(command.Context(), runtime, metadataInput, options)
		},
	}
	addExportFlags(metadataCommand, &metadataInput, "output file or -")
	metadataCommand.Flags().StringVar(&metadataFormat, "format", "json", root.localizer.FlagUsage("format", "metadata format: json, xml, or pixeldata"))

	localizedHelpFlag(command, root.localizer)
	localizedHelpFlag(imageCommand, root.localizer)
	localizedHelpFlag(metadataCommand, root.localizer)
	command.AddCommand(imageCommand, metadataCommand)
	return command
}

func runImageToDICOM(ctx context.Context, runtime Runtime, input, patientName, destination string, recursive, failFast, flatten bool, template map[string]string, reference *dataset.Dataset) error {
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
			dataset, buildErr := convertpkg.NewSecondaryCapture(ctx, imageValue, convertpkg.SecondaryCaptureOptions{PatientName: patientName, StudyUID: studyUID, SeriesUID: seriesUID})
			if buildErr == nil {
				generated := dataset.Clone()
				buildErr = convertpkg.ApplyMetadata(dataset, template)
				if buildErr == nil && reference != nil {
					buildErr = dataset.Merge(reference, true)
				}
				if buildErr == nil {
					buildErr = dataset.Merge(generated, true)
					if buildErr == nil {
						buildErr = convertpkg.ApplyMetadata(dataset, map[string]string{"PatientName": patientName})
					}
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

func runDICOMExport(ctx context.Context, runtime Runtime, input string, options dicomExportOptions) error {
	format := strings.ToLower(options.format)
	if format == "jpg" {
		format = string(convertpkg.JPEG)
	}
	if format != string(convertpkg.PNG) && format != string(convertpkg.JPEG) && format != "json" && format != "xml" && format != "pixeldata" {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("unsupported conversion format %q", options.format))
	}
	if options.frame < 0 {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("--frame must be positive"))
	}
	if options.allFrames && format != string(convertpkg.PNG) && format != string(convertpkg.JPEG) {
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
		return runDICOMExportDirectory(ctx, runtime, input, format, options)
	}
	return runDICOMExportFile(ctx, runtime, input, format, options, options.destination)
}

func runDICOMExportDirectory(ctx context.Context, runtime Runtime, input, format string, options dicomExportOptions) error {
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
	summary := app.Run(entries, options.failFast, func(path string) error {
		return runDICOMExportFile(ctx, runtime, path, format, options, destinationForDICOMExport(path, input, destination, format, options))
	})
	if summary.Failed > 0 {
		return apperr.Wrap(apperr.KindOperation, fmt.Errorf("convert export failed for %d file(s)", summary.Failed))
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
	return strings.TrimSuffix(name, filepath.Ext(name)) + "." + exportExtension(format)
}

func runDICOMExportFile(ctx context.Context, runtime Runtime, input, format string, options dicomExportOptions, destination string) error {
	parsed, err := parser.ParseFile(input)
	if err != nil {
		return apperr.Wrap(apperr.KindOperation, err)
	}
	if format == "json" || format == "xml" || format == "pixeldata" {
		var content []byte
		switch format {
		case "json":
			content, err = convertpkg.ExportJSON(parsed.Dataset)
		case "xml":
			content, err = convertpkg.ExportXML(parsed.Dataset)
		case "pixeldata":
			content, err = convertpkg.ExportPixelData(ctx, parsed.Dataset)
		}
		if err != nil {
			return apperr.Wrap(apperr.KindOperation, err)
		}
		return writeExport(runtime, input, destination, exportExtension(format), content)
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
		content, err := convertpkg.ExportFrame(ctx, parsed.Dataset, parsed.TransferSyntax, frame, convertpkg.ImageFormat(format))
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

func exportExtension(format string) string {
	if format == "pixeldata" {
		return "bin"
	}
	return format
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
	if err := output.NewFile(destination, content); err != nil {
		return apperr.Wrap(apperr.KindInput, err)
	}
	return nil
}

func frameOutputName(input, extension string, frame int) string {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	return fmt.Sprintf("%s-frame-%04d.%s", base, frame+1, extension)
}
