package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

func TestInspectValidateEditSingleFileCommands(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	inspectOutput, err := runCLI(root, "inspect", "--json", "--tag", "PatientID", fixtures.SingleFrame)
	if err != nil || !strings.Contains(inspectOutput, "SYNTHETIC") {
		t.Fatalf("inspect output=%q err=%v", inspectOutput, err)
	}

	rulesPath := filepath.Join(t.TempDir(), "rules.yaml")
	rules := "version: v1\nvalidate:\n  profiles:\n    warning:\n      rules:\n        - when:\n            path: Modality\n            equals: CT\n          assert:\n            path: Modality\n            equals: MR\n          severity: warning\n          message: expected MR\n"
	if err := os.WriteFile(rulesPath, []byte(rules), 0o600); err != nil {
		t.Fatal(err)
	}
	validateOutput, err := runCLI(root, "validate", "--strict", "--rules", rulesPath, "--profile", "warning", fixtures.SingleFrame)
	if err == nil || !strings.Contains(validateOutput, "expected MR") {
		t.Fatalf("validate output=%q err=%v", validateOutput, err)
	}

	edited := filepath.Join(t.TempDir(), "edited.dcm")
	if output, err := runCLI(root, "edit", "--set", "PatientName=EDITED^PATIENT", "--output", edited, fixtures.SingleFrame); err != nil {
		t.Fatalf("edit output=%q err=%v", output, err)
	}
	if _, err := parser.ParseFile(edited); err != nil {
		t.Fatalf("edited output is not a readable DICOM file: %v", err)
	}
}

func runCLI(root string, args ...string) (string, error) {
	commandArgs := append([]string{"run", "./cmd/dicom-cli"}, args...)
	command := exec.Command("go", commandArgs...)
	command.Dir = root
	output, err := command.CombinedOutput()
	return string(output), err
}
