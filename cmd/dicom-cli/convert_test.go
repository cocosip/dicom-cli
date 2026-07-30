package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

func TestExecuteConvertImageAndJSONUseSharedDICOMExport(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pngPath := filepath.Join(t.TempDir(), "frame.png")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--format", "png", "--output", pngPath, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("convert image exit code = %d, want 0", code)
	}
	pngContent, err := os.ReadFile(pngPath)
	if err != nil || !bytes.HasPrefix(pngContent, []byte("\x89PNG")) {
		t.Fatalf("PNG output = %q, err=%v", pngContent, err)
	}

	jsonPath := filepath.Join(t.TempDir(), "metadata.json")
	runtime, _, _ = testRuntime()
	if code := Execute([]string{"convert", "--to", "json", "--output", jsonPath, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("convert --to json exit code = %d, want 0", code)
	}
	jsonContent, err := os.ReadFile(jsonPath)
	if err != nil || !strings.Contains(string(jsonContent), `"summary"`) {
		t.Fatalf("JSON output = %q, err=%v", jsonContent, err)
	}
}

func TestExecuteConvertDICOMGroupsDirectoryUIDs(t *testing.T) {
	input := t.TempDir()
	for _, name := range []string{"one.png", "two.png"} {
		file, err := os.Create(filepath.Join(input, name))
		if err != nil {
			t.Fatal(err)
		}
		imageValue := image.NewGray(image.Rect(0, 0, 1, 1))
		imageValue.SetGray(0, 0, color.Gray{Y: 127})
		if err := png.Encode(file, imageValue); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "dicom", "--patient-name", "SYNTHETIC^PATIENT", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("convert dicom exit code = %d, want 0", code)
	}
	first, err := parser.ParseFile(filepath.Join(output, "one.dcm"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := parser.ParseFile(filepath.Join(output, "two.dcm"))
	if err != nil {
		t.Fatal(err)
	}
	firstStudy, _ := first.Dataset.GetString(tag.StudyInstanceUID)
	secondStudy, _ := second.Dataset.GetString(tag.StudyInstanceUID)
	firstSeries, _ := first.Dataset.GetString(tag.SeriesInstanceUID)
	secondSeries, _ := second.Dataset.GetString(tag.SeriesInstanceUID)
	firstSOP, _ := first.Dataset.GetString(tag.SOPInstanceUID)
	secondSOP, _ := second.Dataset.GetString(tag.SOPInstanceUID)
	if firstStudy == "" || firstStudy != secondStudy || firstSeries == "" || firstSeries != secondSeries || firstSOP == secondSOP {
		t.Fatalf("UIDs first=(%q,%q,%q) second=(%q,%q,%q)", firstStudy, firstSeries, firstSOP, secondStudy, secondSeries, secondSOP)
	}
	if first.TransferSyntax != transfer.ExplicitVRLittleEndian {
		t.Fatalf("transfer syntax = %s, want Explicit VR Little Endian", first.TransferSyntax.UID())
	}
}

func TestExecuteConvertToDICOMUsesImageEncapsulationPath(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.png")
	file, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "--to", "dicom", "--patient-name", "SYNTHETIC^PATIENT", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("convert --to dicom exit code = %d, want 0", code)
	}
	if _, err := parser.ParseFile(output); err != nil {
		t.Fatalf("parse output: %v", err)
	}
}

func TestExecuteConvertDICOMMergesTemplateReferenceAndCLI(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.png")
	file, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(rulesPath, []byte("version: v1\ndicom_templates:\n  source:\n    tags:\n      PatientName: TEMPLATE^PATIENT\n      PatientID: template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reference := filepath.Join(t.TempDir(), "reference.dcm")
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reference, content, 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "dicom", "--rules", rulesPath, "--template", "source", "--reference", reference, "--patient-name", "CLI^PATIENT", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("convert dicom exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := parsed.Dataset.GetString(tag.PatientName); got != "CLI^PATIENT" {
		t.Fatalf("PatientName = %q, want CLI override", got)
	}
	if got, _ := parsed.Dataset.GetString(tag.PatientID); got != "SYNTHETIC" {
		t.Fatalf("PatientID = %q, want reference override", got)
	}
}

func TestExecuteConvertImageRejectsMultipleFramesToStdout(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--all-frames", "--output", "-", fixtures.MultiFrame}, runtime); code != 2 {
		t.Fatalf("convert image --all-frames stdout exit code = %d, want 2", code)
	}
}

func TestExecuteConvertImageAllFramesUsesDeterministicNames(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "frames")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--all-frames", "--output", output, fixtures.MultiFrame}, runtime); code != 0 {
		t.Fatalf("convert image exit code = %d, want 0", code)
	}
	for _, name := range []string{"multi-frame-frame-0001.png", "multi-frame-frame-0002.png"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("frame output %s: %v", name, err)
		}
	}
}
