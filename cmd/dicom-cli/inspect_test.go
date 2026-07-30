package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
)

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
	runtime, _, _ = testRuntime()
	if code := Execute([]string{"inspect", directory}, runtime); code != 2 {
		t.Fatalf("inspect directory exit code = %d, want 2", code)
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
