package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	convertpkg "github.com/cocosip/dicom-cli/internal/convert"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

type dicomExportOptions struct {
	format           string
	destination      string
	frame            int
	allFrames        bool
	includePixelData bool
}

func newConvertCommand(runtime Runtime, _ *rootOptions) *cobra.Command {
	options := dicomExportOptions{format: "png"}
	command := &cobra.Command{
		Use:   "convert <input>",
		Short: "Convert DICOM images and metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDICOMExport(runtime, args[0], options)
		},
	}
	command.Flags().StringVar(&options.format, "to", "", "output format: png, jpeg, or json")
	command.Flags().StringVarP(&options.destination, "output", "o", "", "output file, directory, or -")
	command.Flags().IntVar(&options.frame, "frame", 0, "one-based frame number")
	command.Flags().BoolVar(&options.allFrames, "all-frames", false, "export every image frame")
	command.Flags().BoolVar(&options.includePixelData, "include-pixel-data", false, "include PixelData bytes in JSON")

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

	command.AddCommand(imageCommand, jsonCommand)
	return command
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
