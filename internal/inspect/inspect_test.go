package inspect

import (
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

func TestBuildDefaultReportIncludesSyntheticSummary(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}

	report, err := Build(fixtures.SingleFrame, parsed, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Patient.ID != "SYNTHETIC" {
		t.Fatalf("Patient.ID = %q, want SYNTHETIC", report.Patient.ID)
	}
	if report.Study.Modality != "CT" {
		t.Fatalf("Study.Modality = %q, want CT", report.Study.Modality)
	}
	if report.Pixel.Rows != 1 || report.Pixel.Columns != 2 || report.Pixel.Bytes != 4 {
		t.Fatalf("Pixel = %#v, want rows=1 columns=2 bytes=4", report.Pixel)
	}
}

func TestBuildResolvesKeywordAndHexTagPaths(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.Sequence)
	if err != nil {
		t.Fatal(err)
	}

	report, err := Build(fixtures.Sequence, parsed, Options{Tags: []string{"PatientName", "0040,A730[0].0040,A160"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tags) != 2 {
		t.Fatalf("len(Tags) = %d, want 2", len(report.Tags))
	}
	if report.Tags[0].Value != "SYNTHETIC^PATIENT" || report.Tags[1].Value != "synthetic nested value" {
		t.Fatalf("Tags = %#v", report.Tags)
	}
}
