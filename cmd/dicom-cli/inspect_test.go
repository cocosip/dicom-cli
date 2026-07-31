package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
)

func TestExecuteInspectUsesConfiguredLanguageForTextReport(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dicom-cli.yaml")
	if err := os.WriteFile(path, []byte("version: v1\nlanguage: zh-CN\ntargets: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, stdout, stderr := testRuntime()

	if code := Execute([]string{"--config", path, "inspect", fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("inspect exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "[文件]") || !strings.Contains(stdout.String(), "传输语法：") {
		t.Fatalf("localized inspect output = %q", stdout.String())
	}
}

func TestExecuteInspectWritesJSONAndRejectsDirectory(t *testing.T) {
	directory := t.TempDir()
	fixtures, err := testutil.CreateDICOMFixtures(directory)
	if err != nil {
		t.Fatal(err)
	}
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"inspect", "--json", "--tag", "PatientID", "--tag", "0040,A730[0].0040,A160", fixtures.Sequence}, runtime); code != 0 {
		t.Fatalf("inspect exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "\"SYNTHETIC\"") || !strings.Contains(stdout.String(), "synthetic nested value") {
		t.Fatalf("inspect output = %s", stdout.String())
	}
	for _, expected := range []string{"\"date\": \"20260730\"", "\"accession_number\": \"ACC-001\"", "\"pixel_spacing\": \"0.5\\\\0.5\"", "\"window_width\": \"400\""} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("inspect JSON is missing %q: %s", expected, stdout.String())
		}
	}
	runtime, _, _ = testRuntime()
	if code := Execute([]string{"inspect", directory}, runtime); code != 2 {
		t.Fatalf("inspect directory exit code = %d, want 2", code)
	}
}

func TestExecuteInspectWritesGroupedDefaultTextSummary(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"inspect", fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("inspect exit code = %d, want 0", code)
	}
	for _, expected := range []string{
		"[File]\n  Path: ",
		"[Patient]\n  Name: SYNTHETIC^PATIENT\n  ID: SYNTHETIC\n  Birth Date: 19800102\n  Sex: F",
		"[Study]\n  Instance UID: 1.2.826.0.1.3680043.10.5432\n  Modality: CT\n  Date: 20260730\n  Time: 123456\n  Accession Number: ACC-001\n  Description: Synthetic CT study",
		"[Series]\n  Instance UID: 1.2.826.0.1.3680043.10.5432.1\n  Number: 7\n  Description: Synthetic axial\n  Body Part: CHEST\n  Laterality: R\n  Protocol: Chest routine",
		"[Instance]\n  SOP Class UID: 1.2.840.10008.5.1.4.1.1.2\n  SOP Instance UID: ",
		"[Pixel]\n  Rows: 1\n  Columns: 2\n  Frames: 1\n  Bytes: 4\n  Samples Per Pixel: 1\n  Photometric Interpretation: MONOCHROME2",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("inspect text is missing %q: %s", expected, stdout.String())
		}
	}
}

func TestExecuteInspectLoadsProfileAndWritesReportFile(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	rulesPath := filepath.Join(directory, "rules.yaml")
	if err := os.WriteFile(rulesPath, []byte("version: v1\ninspect:\n  profiles:\n    patient:\n      tags: [PatientName]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(directory, "inspect.json")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"inspect", "--rules", rulesPath, "--profile", "patient", "--json", "--output", reportPath, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("inspect exit code = %d, want 0", code)
	}
	content, err := os.ReadFile(reportPath)
	if err != nil || !strings.Contains(string(content), "SYNTHETIC^PATIENT") {
		t.Fatalf("report = %q, err=%v", content, err)
	}
}
