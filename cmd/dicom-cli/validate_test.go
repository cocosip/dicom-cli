package main

import (
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
	"golang.org/x/text/encoding/unicode"
)

func TestExecuteValidateReturnsThreeForStrictProfileWarning(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	content := "version: v1\nvalidate:\n  profiles:\n    warn:\n      rules:\n        - when:\n            path: Modality\n            equals: CT\n          assert:\n            path: Modality\n            equals: MR\n          severity: warning\n          message: expected MR\n"
	if err := os.WriteFile(rulesPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, stdout, _ := testRuntime()
	reportPath := filepath.Join(t.TempDir(), "validate.json")
	if code := Execute([]string{"validate", "--json", "--strict", "--rules", rulesPath, "--profile", "warn", "--output", reportPath, fixtures.SingleFrame}, runtime); code != 3 {
		t.Fatalf("validate exit code = %d, want 3", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("validate stdout = %s, want empty when writing a report", stdout.String())
	}
	reportContent, err := os.ReadFile(reportPath)
	if err != nil || !strings.Contains(string(reportContent), "expected MR") {
		t.Fatalf("validate report = %q, err=%v", reportContent, err)
	}
}

func TestExecuteValidateCharsetCheck(t *testing.T) {
	fixture := writeCharsetMismatchFixture(t)

	runtime, stdout, stderr := testRuntime()
	if code := Execute([]string{"validate", fixture}, runtime); code != 0 {
		t.Fatalf("default validate exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "valid\n" {
		t.Fatalf("default validate output = %q, want valid", stdout.String())
	}

	runtime, stdout, stderr = testRuntime()
	if code := Execute([]string{"validate", "--charset-check", fixture}, runtime); code != 0 {
		t.Fatalf("charset check exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "warning dicom-cli.charset SpecificCharacterSet:") || !strings.Contains(stdout.String(), "recommended=ISO_IR 192") {
		t.Fatalf("charset check output = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "张三") || strings.Contains(stdout.String(), "示例医院") {
		t.Fatalf("charset check leaked text: %q", stdout.String())
	}

	runtime, stdout, stderr = testRuntime()
	if code := Execute([]string{"validate", "--charset-check", "--json", fixture}, runtime); code != 0 {
		t.Fatalf("JSON charset check exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"source": "dicom-cli.charset"`) || strings.Contains(stdout.String(), "张三") || strings.Contains(stdout.String(), "示例医院") {
		t.Fatalf("JSON charset check output = %q", stdout.String())
	}

	runtime, stdout, stderr = testRuntime()
	if code := Execute([]string{"validate", "--charset-check", "--strict", fixture}, runtime); code != 3 {
		t.Fatalf("strict charset check exit code = %d, want 3, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "warning dicom-cli.charset") {
		t.Fatalf("strict charset check output = %q", stdout.String())
	}
}

func TestExecuteValidateCharsetCheckHelp(t *testing.T) {
	runtime, stdout, stderr := testRuntime()
	if code := Execute([]string{"validate", "--help"}, runtime); code != 0 {
		t.Fatalf("English help exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "--charset-check") || !strings.Contains(stdout.String(), "raw text bytes") {
		t.Fatalf("English help = %q", stdout.String())
	}

	configPath := filepath.Join(t.TempDir(), "dicom-cli.yaml")
	if err := os.WriteFile(configPath, []byte("version: v1\nlanguage: zh-CN\ntargets: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, stdout, stderr = testRuntime()
	if code := Execute([]string{"--config", configPath, "validate", "--help"}, runtime); code != 0 {
		t.Fatalf("Chinese help exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "检测特定字符集与原始文本字节是否不一致") {
		t.Fatalf("Chinese help = %q", stdout.String())
	}
}

func writeCharsetMismatchFixture(t *testing.T) string {
	t.Helper()
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []element.Element{
		element.NewString(tag.SpecificCharacterSet, vr.CS, []string{"GB18030"}),
		element.NewStringWithEncoding(tag.PatientName, vr.PN, []string{"张三"}, unicode.UTF8),
		element.NewStringWithEncoding(tag.InstitutionName, vr.LO, []string{"示例医院"}, unicode.UTF8),
	} {
		if err := parsed.Dataset.AddOrUpdate(value); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "charset-mismatch.dcm")
	if err := writer.WriteFile(path, parsed.Dataset, writer.WithTransferSyntax(transfer.ExplicitVRLittleEndian)); err != nil {
		t.Fatal(err)
	}
	return path
}
