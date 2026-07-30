package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
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
	if code := Execute([]string{"anonymize", "--profile", "basic", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
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
	if patientName, _ := parsed.Dataset.GetString(tag.PatientName); patientName == "SYNTHETIC^PATIENT" {
		t.Fatal("output PatientName was not anonymized")
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
	if patientName, _ := parsed.Dataset.GetString(tag.PatientName); patientName == "SYNTHETIC^PATIENT" {
		t.Fatal("default Basic profile did not anonymize PatientName")
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
	if code := Execute([]string{"anonymize", "--profile", "basic", "--output", output, "--report", report, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	if strings.Contains(stdout.String(), "SYNTHETIC^PATIENT") {
		t.Fatalf("stdout leaked report data: %s", stdout.String())
	}
	content, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "SYNTHETIC^PATIENT") || !strings.Contains(string(content), "uid_mappings") {
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
	if code := Execute([]string{"anonymize", "--profile", "basic", "--recursive", "--output", output, input}, runtime); code != 1 {
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

func TestExecuteAnonymizeLoadsExternalProfileOptionsAndFilter(t *testing.T) {
	input := t.TempDir()
	fixtures, err := testutil.CreateDICOMFixtures(input)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.MultiFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.Dataset.AddOrUpdate(element.NewString(tag.Modality, vr.CS, []string{"MR"})); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteFile(fixtures.MultiFrame, parsed.Dataset, writer.WithTransferSyntax(parsed.TransferSyntax)); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	rulesContent := "version: v1\nfilters:\n  ct-only:\n    path: Modality\n    equals: CT\nanonymize:\n  profiles:\n    research:\n      filter: ct-only\n      options: [retain-uids]\n      rules:\n        - path: PatientName\n          action: replace\n          value: ANON^RESEARCH\n"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output")
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--rules", rulesPath, "--profile", "research", "--output", output, input}, runtime); code != 1 {
		t.Fatalf("anonymize exit code = %d, want 1 for corrupt input; stdout=%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "skipped=1") {
		t.Fatalf("filter summary = %s", stdout.String())
	}
	result, err := parser.ParseFile(filepath.Join(output, "single-frame.dcm"))
	if err != nil {
		t.Fatal(err)
	}
	if patientName, _ := result.Dataset.GetString(tag.PatientName); patientName != "ANON^RESEARCH" {
		t.Fatalf("PatientName = %q, want rule replacement", patientName)
	}
	if studyUID, _ := result.Dataset.GetString(tag.StudyInstanceUID); studyUID != "1.2.826.0.1.3680043.10.5432" {
		t.Fatalf("StudyInstanceUID = %q, want retained UID", studyUID)
	}
	if _, err := os.Stat(filepath.Join(output, "multi-frame.dcm")); !os.IsNotExist(err) {
		t.Fatalf("nonmatching file was anonymized, err=%v", err)
	}
}

func TestExecuteAnonymizeLoadsExplicitExternalBasicProfile(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	rulesContent := "version: v1\nanonymize:\n  profiles:\n    basic:\n      rules:\n        - path: PatientName\n          action: replace\n          value: ANON^CUSTOM\n"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "anonymized.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"anonymize", "--rules", rulesPath, "--profile", "basic", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if patientName, _ := parsed.Dataset.GetString(tag.PatientName); patientName != "ANON^CUSTOM" {
		t.Fatalf("PatientName = %q, want custom external basic profile", patientName)
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
	if code := Execute([]string{"anonymize", "--profile", "basic", "--fail-fast", "--output", output, input}, runtime); code != 1 {
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
	if code := Execute([]string{"anonymize", "--profile", "basic", "--output", "-", fixtures.SingleFrame}, runtime); code != 0 {
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
	if code := Execute([]string{"anonymize", "--profile", "basic", "--recursive", "--flatten", "--output", output, input}, runtime); code != 0 {
		t.Fatalf("anonymize exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(output, "nested.dcm")); err != nil {
		t.Fatalf("flattened output missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "child", "nested.dcm")); !os.IsNotExist(err) {
		t.Fatalf("output retained child directory, err=%v", err)
	}
}
