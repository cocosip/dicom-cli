package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

func TestAnonymizeCommandCreatesReadableNewDICOM(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "anonymized.dcm")
	if output, err := runCLI(root, "anonymize", "--profile", "basic", "--output", outputPath, fixtures.SingleFrame); err != nil {
		t.Fatalf("anonymize output=%q err=%v", output, err)
	}
	after, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("anonymize modified its input")
	}
	parsed, err := parser.ParseFile(outputPath)
	if err != nil {
		t.Fatalf("anonymized output is not a readable DICOM file: %v", err)
	}
	if patientName, _ := parsed.Dataset.GetString(tag.PatientName); patientName == "SYNTHETIC^PATIENT" {
		t.Fatal("anonymized output retained PatientName")
	}
}
