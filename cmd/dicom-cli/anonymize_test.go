package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	goanonymizer "github.com/cocosip/go-dicom/pkg/dicom/anonymizer"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

func TestExecuteAnonymizeWritesNewFileAndProtectsDefaultSummary(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "anonymized.dcm")
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	if strings.Contains(stdout.String(), "SYNTHETIC^PATIENT") {
		t.Fatalf("default summary leaked PatientName: %s", stdout.String())
	}
	after, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("input DICOM was modified")
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	source, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := goanonymizer.NewAnonymizer(nil).Anonymize(source.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	expectedPatientName, _ := expected.GetString(tag.PatientName)
	if got, _ := parsed.Dataset.GetString(tag.PatientName); got != expectedPatientName {
		t.Fatalf("output PatientName = %q, want go-dicom Basic Profile result", got)
	}
}

func TestExecuteAnonymizeUsesBuiltInBasicProfileByDefault(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "anonymized.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	source, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := goanonymizer.NewAnonymizer(nil).Anonymize(source.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	expectedPatientName, _ := expected.GetString(tag.PatientName)
	if got, _ := parsed.Dataset.GetString(tag.PatientName); got != expectedPatientName {
		t.Fatalf("default Basic profile PatientName = %q, want go-dicom result", got)
	}
}

func TestExecuteAnonymizeRejectsCustomRuleConfiguration(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "anonymized.dcm")
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "profile", args: []string{"anonymize", "--profile", "basic", "--output", output, fixtures.SingleFrame}},
		{name: "rules", args: []string{"--rules", "ignored.yaml", "anonymize", "--output", output, fixtures.SingleFrame}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, _, _ := testRuntime()
			if code := Execute(test.args, runtime); code != 2 {
				t.Fatalf("anonymize exit code = %d, want 2", code)
			}
		})
	}
}

func TestExecuteAnonymizeWritesDetailedReportOnlyToFile(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "anonymized.dcm")
	report := filepath.Join(t.TempDir(), "report.json")
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--output", output, "--report", report, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	if strings.Contains(stdout.String(), "SYNTHETIC^PATIENT") {
		t.Fatalf("stdout leaked report data: %s", stdout.String())
	}
	content, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "ACC-001") || !strings.Contains(string(content), "uid_mappings") {
		t.Fatalf("detailed report = %s", content)
	}
}

func TestExecuteAnonymizeDirectoryPreservesTreeAndContinuesAfterBadInput(t *testing.T) {
	input := t.TempDir()
	fixtures, err := testutil.CreateDICOMFixtures(input)
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(input, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(fixtures.Sequence, filepath.Join(child, "nested.dcm")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--recursive", "--output", output, input}, runtime); code != 1 {
		t.Fatalf("anonymize directory exit code = %d, want 1 for corrupt input", code)
	}
	if !strings.Contains(stdout.String(), "processed") || !strings.Contains(stdout.String(), "failed") {
		t.Fatalf("summary = %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(output, "single-frame.dcm")); err != nil {
		t.Fatalf("top-level output missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "child", "nested.dcm")); err != nil {
		t.Fatalf("nested output missing: %v", err)
	}
}

func TestExecuteAnonymizeFailFastStopsAfterFirstBadFile(t *testing.T) {
	input := t.TempDir()
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "a-bad.dcm"), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "z-valid.dcm"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--fail-fast", "--output", output, input}, runtime); code != 1 {
		t.Fatalf("anonymize exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "processed=0") || !strings.Contains(stdout.String(), "failed=1") {
		t.Fatalf("fail-fast summary = %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(output, "z-valid.dcm")); !os.IsNotExist(err) {
		t.Fatalf("file after first failure was processed, err=%v", err)
	}
}

func TestExecuteAnonymizeWritesBinaryStdoutAndSummaryToStderr(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, stdout, stderr := testRuntime()
	if code := Execute([]string{"anonymize", "--output", "-", fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Fatal("binary stdout is empty")
	}
	if strings.Contains(stdout.String(), "scanned=") {
		t.Fatal("binary stdout contains text summary")
	}
	if !strings.Contains(stderr.String(), "scanned=1") {
		t.Fatalf("stderr summary = %s", stderr.String())
	}
}

func TestExecuteAnonymizeFlattensDirectoryOutput(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := t.TempDir()
	child := filepath.Join(input, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(fixtures.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "nested.dcm"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--recursive", "--flatten", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(output, "nested.dcm")); err != nil {
		t.Fatalf("flattened output missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "child", "nested.dcm")); !os.IsNotExist(err) {
		t.Fatalf("output retained child directory, err=%v", err)
	}
}
