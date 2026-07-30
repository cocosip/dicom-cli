package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
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
