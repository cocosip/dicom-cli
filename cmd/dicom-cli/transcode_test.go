package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

func TestExecuteTranscodeFormatsListsRuntimeCodecsAndExperimentalHTJ2K(t *testing.T) {
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"transcode", "formats", "--json"}, runtime); code != 0 {
		t.Fatalf("transcode formats exit code = %d, want 0", code)
	}
	for _, want := range []string{"explicit-vr-little-endian", "JPEG 2000 Lossless", "1.2.840.10008.1.2.4.201", `"experimental":true`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("formats output does not contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestExecuteTranscodeHelpExplainsTargetTransferSyntax(t *testing.T) {
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"transcode", "--help"}, runtime); code != 0 {
		t.Fatalf("transcode --help exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"transcode --input <file-or-directory>",
		"--input string",
		"input DICOM file or directory",
		"output DICOM file or directory",
		"--to",
		"standard name, short name, or UID",
		"JPEG 2000 Lossless",
		"jpeg2000-lossless",
		"1.2.840.10008.1.2.4.90",
		"transcode formats",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("transcode help does not contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestExecuteTranscodeDirectoryWritesAllDICOMFiles(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := t.TempDir()
	for _, name := range []string{"one.dcm", "two.dcm"} {
		content, err := os.ReadFile(fixtures.SingleFrame)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(input, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--to", transfer.ImplicitVRLittleEndian.UID().UID(), "--output", output, input}, runtime); code != 0 {
		t.Fatalf("directory transcode exit code = %d, want 0", code)
	}
	for _, name := range []string{"one.dcm", "two.dcm"} {
		parsed, err := parser.ParseFile(filepath.Join(output, name))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if parsed.TransferSyntax != transfer.ImplicitVRLittleEndian {
			t.Fatalf("%s transfer syntax = %s", name, parsed.TransferSyntax.UID())
		}
	}
}

func TestExecuteTranscodeDirectorySkipsFilesOutsideFilter(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := t.TempDir()
	content, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "one.dcm"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	rules := "version: v1\nfilters:\n  exclude_all:\n    path: PatientID\n    equals: OTHER\n"
	if err := os.WriteFile(rulesPath, []byte(rules), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--rules", rulesPath, "--filter", "exclude_all", "--to", "implicit-vr-little-endian", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("filtered directory transcode exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(output, "one.dcm")); !os.IsNotExist(err) {
		t.Fatalf("filtered output exists or stat failed: %v", err)
	}
}

func TestExecuteTranscodeRoundTripsRLE(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	compressed := filepath.Join(t.TempDir(), "compressed.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--to", "rle", "--output", compressed, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("RLE transcode exit code = %d, want 0", code)
	}
	roundTrip := filepath.Join(t.TempDir(), "roundtrip.dcm")
	runtime, _, _ = testRuntime()
	if code := Execute([]string{"transcode", "--to", "explicit-vr-little-endian", "--output", roundTrip, compressed}, runtime); code != 0 {
		t.Fatalf("RLE decode exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := parsed.Dataset.GetString(tag.PatientID); got != "SYNTHETIC" {
		t.Fatalf("PatientID = %q, want retained value", got)
	}
}

func TestExecuteTranscodeWritesRequestedSyntax(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--to", "implicit-vr-little-endian", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("transcode exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.TransferSyntax != transfer.ImplicitVRLittleEndian {
		t.Fatalf("transfer syntax = %s, want Implicit VR Little Endian", parsed.TransferSyntax.UID())
	}
	if parsed.FileMetaInformation == nil {
		t.Fatal("file meta information is missing")
	}
	if got, _ := parsed.FileMetaInformation.Dataset().GetString(tag.TransferSyntaxUID); got != transfer.ImplicitVRLittleEndian.UID().UID() {
		t.Fatalf("file meta TransferSyntaxUID = %q", got)
	}
}

func TestExecuteTranscodeAcceptsExplicitInputFlag(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--input", fixtures.SingleFrame, "--to", "implicit-vr-little-endian", "--output", output}, runtime); code != 0 {
		t.Fatalf("transcode --input exit code = %d, want 0", code)
	}
	if _, err := parser.ParseFile(output); err != nil {
		t.Fatalf("parse transcode --input output: %v", err)
	}
}

func TestExecuteTranscodeAcceptsStandardTransferSyntaxName(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--input", fixtures.SingleFrame, "--to", "JPEG 2000 Lossless", "--output", output}, runtime); code != 0 {
		t.Fatalf("transcode standard name exit code = %d, want 0", code)
	}
	if _, err := parser.ParseFile(output); err != nil {
		t.Fatalf("parse transcode standard name output: %v", err)
	}
}

func TestExecuteTranscodeDirectoryRecursivelyPreservesRelativePaths(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := t.TempDir()
	nested := filepath.Join(input, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "one.dcm"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--recursive", "--to", "implicit-vr-little-endian", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("recursive transcode exit code = %d, want 0", code)
	}
	if _, err := parser.ParseFile(filepath.Join(output, "nested", "one.dcm")); err != nil {
		t.Fatalf("parse recursive output: %v", err)
	}
}
