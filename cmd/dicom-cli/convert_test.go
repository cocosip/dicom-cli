package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

func TestExecuteConvertImageAndJSONExportDICOM(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pngPath := filepath.Join(t.TempDir(), "frame.png")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--input", fixtures.SingleFrame, "--format", "png", "--output", pngPath}, runtime); code != 0 {
		t.Fatalf("convert image exit code = %d, want 0", code)
	}
	pngContent, err := os.ReadFile(pngPath)
	if err != nil || !bytes.HasPrefix(pngContent, []byte("\x89PNG")) {
		t.Fatalf("PNG output = %q, err=%v", pngContent, err)
	}

	jsonPath := filepath.Join(t.TempDir(), "metadata.json")
	runtime, _, _ = testRuntime()
	if code := Execute([]string{"convert", "json", "-i", fixtures.SingleFrame, "--output", jsonPath}, runtime); code != 0 {
		t.Fatalf("convert json exit code = %d, want 0", code)
	}
	jsonContent, err := os.ReadFile(jsonPath)
	if err != nil || !strings.Contains(string(jsonContent), `"summary"`) {
		t.Fatalf("JSON output = %q, err=%v", jsonContent, err)
	}
}

func TestExecuteConvertRejectsMissingOrPositionalInput(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"convert", "image"},
		{"convert", "json"},
		{"convert", "image", fixtures.SingleFrame},
		{"convert", "json", fixtures.SingleFrame},
	} {
		runtime, _, _ := testRuntime()
		if code := Execute(args, runtime); code != 2 {
			t.Fatalf("Execute(%v) = %d, want 2", args, code)
		}
	}
}

func TestExecuteEncapsulateImageGroupsDirectoryUIDs(t *testing.T) {
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
	sourcePath := filepath.Join(input, "one.png")
	sourceBefore, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"encapsulate", "image", "--patient-name", "SYNTHETIC^PATIENT", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("encapsulate image exit code = %d, want 0", code)
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
	sourceAfter, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sourceBefore, sourceAfter) {
		t.Fatal("source image changed during conversion")
	}
}

func TestExecuteEncapsulateImageUsesDefaultTransferSyntax(t *testing.T) {
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
	if code := Execute([]string{"encapsulate", "image", "--patient-name", "SYNTHETIC^PATIENT", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("encapsulate image exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if parsed.TransferSyntax != transfer.ExplicitVRLittleEndian {
		t.Fatalf("transfer syntax = %s, want Explicit VR Little Endian", parsed.TransferSyntax.UID())
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
	referenceDataset, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := referenceDataset.Dataset.AddOrUpdate(element.NewString(tag.StudyDescription, vr.LO, []string{"REFERENCE STUDY"})); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteFile(reference, referenceDataset.Dataset, writer.WithTransferSyntax(referenceDataset.TransferSyntax)); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"encapsulate", "image", "--rules", rulesPath, "--template", "source", "--reference", reference, "--patient-name", "CLI^PATIENT", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("encapsulate image exit code = %d, want 0", code)
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
	if got, _ := parsed.Dataset.GetString(tag.StudyDescription); got != "REFERENCE STUDY" {
		t.Fatalf("StudyDescription = %q, want reference metadata", got)
	}
}

func TestExecuteConvertImageRejectsMultipleFramesToStdout(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--input", fixtures.MultiFrame, "--all-frames", "--output", "-"}, runtime); code != 2 {
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
	if code := Execute([]string{"convert", "image", "--input", fixtures.MultiFrame, "--all-frames", "--output", output}, runtime); code != 0 {
		t.Fatalf("convert image exit code = %d, want 0", code)
	}
	for _, name := range []string{"multi-frame-frame-0001.png", "multi-frame-frame-0002.png"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("frame output %s: %v", name, err)
		}
	}
}

func TestExecuteConvertImageAllFramesUsesDefaultConvertDirectory(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workingDirectory := t.TempDir()
	runtime, _, _ := testRuntime()
	runtime.Getwd = func() (string, error) { return workingDirectory, nil }
	if code := Execute([]string{"convert", "image", "--input", fixtures.MultiFrame, "--all-frames"}, runtime); code != 0 {
		t.Fatalf("convert image exit code = %d, want 0", code)
	}
	for _, name := range []string{"multi-frame-frame-0001.png", "multi-frame-frame-0002.png"} {
		if _, err := os.Stat(filepath.Join(workingDirectory, "convert", name)); err != nil {
			t.Fatalf("default frame output %s: %v", name, err)
		}
	}
}

func TestExecuteConvertImageExportsDICOMDirectory(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "input")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.dcm", "two.dcm"} {
		content, readErr := os.ReadFile(fixtures.SingleFrame)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(input, name), content, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--input", input, "--output", output}, runtime); code != 0 {
		t.Fatalf("convert image directory exit code = %d, want 0", code)
	}
	for _, name := range []string{"one.png", "two.png"} {
		content, readErr := os.ReadFile(filepath.Join(output, name))
		if readErr != nil || !bytes.HasPrefix(content, []byte("\x89PNG")) {
			t.Fatalf("PNG output %s = %q, err=%v", name, content, readErr)
		}
	}
}

func TestExecuteEncapsulateImageDisambiguatesSameStemImageNames(t *testing.T) {
	input := t.TempDir()
	imageValue := image.NewGray(image.Rect(0, 0, 1, 1))
	pngFile, err := os.Create(filepath.Join(input, "image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(pngFile, imageValue); err != nil {
		t.Fatal(err)
	}
	if err := pngFile.Close(); err != nil {
		t.Fatal(err)
	}
	jpegFile, err := os.Create(filepath.Join(input, "image.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(jpegFile, imageValue, nil); err != nil {
		t.Fatal(err)
	}
	if err := jpegFile.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"encapsulate", "image", "--patient-name", "SYNTHETIC^PATIENT", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("encapsulate image exit code = %d, want 0", code)
	}
	for _, name := range []string{"image.dcm", "image-1.dcm"} {
		if _, err := parser.ParseFile(filepath.Join(output, name)); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
}

func TestExecuteConvertHelpListsOnlyDICOMExportSubcommands(t *testing.T) {
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"convert", "--help"}, runtime); code != 0 {
		t.Fatalf("convert --help exit code = %d, want 0", code)
	}
	for _, want := range []string{"image", "json"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("convert help does not contain %q:\n%s", want, stdout.String())
		}
	}
	for _, unwanted := range []string{"\n  dicom ", "--to"} {
		if strings.Contains(stdout.String(), unwanted) {
			t.Fatalf("convert help contains removed %q:\n%s", unwanted, stdout.String())
		}
	}
}
