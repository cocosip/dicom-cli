package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/files"
	"github.com/cocosip/dicom-cli/internal/output"
)

func requireRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("%q is a directory; this command requires one DICOM file", path))
	}
	return nil
}

func writeReport(runtime Runtime, input, destination string, content []byte) error {
	if destination == "" {
		_, err := runtime.Stdout.Write(content)
		return err
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
	return output.File(destination, content)
}

func newOutputPath(runtime Runtime, input, destination string) (string, error) {
	if destination == "" {
		workingDirectory, err := runtime.Getwd()
		if err != nil {
			return "", err
		}
		root := files.DefaultOutputDirectory(workingDirectory, "edit")
		path, err := files.OutputPath(input, input, root, false)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		return path, nil
	}
	inputPath, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	outputPath, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if filepath.Clean(inputPath) == filepath.Clean(outputPath) {
		return "", apperr.Wrap(apperr.KindInput, fmt.Errorf("output path is the input path"))
	}
	if _, err := os.Stat(destination); err == nil {
		return "", apperr.Wrap(apperr.KindInput, fmt.Errorf("output file %q already exists", destination))
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	return destination, nil
}
