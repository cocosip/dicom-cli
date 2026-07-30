package dicom

import (
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

func TestParseTagPathAcceptsKeywordHexAndSequenceIndex(t *testing.T) {
	for _, path := range []string{"PatientName", "0010,0010", "(0010,0010)", "ContentSequence[0].TextValue", "0040,A730[0].0040,A160"} {
		if _, err := ParseTagPath(path); err != nil {
			t.Fatalf("ParseTagPath(%q) error = %v", path, err)
		}
	}
}

func TestResolveElementReadsKeywordHexAndNestedSequence(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"PatientName", "0010,0020", "ContentSequence[0].TextValue", "0040,A730[0].0040,A160"} {
		element, err := ResolveElement(parsed.Dataset, path)
		if err != nil {
			t.Fatalf("ResolveElement(%q) error = %v", path, err)
		}
		if element == nil {
			t.Fatalf("ResolveElement(%q) = nil", path)
		}
	}
	parsed.Dataset.Remove(tag.StudyDescription)
	if _, err := ResolveElement(parsed.Dataset, "StudyDescription"); err == nil {
		t.Fatal("ResolveElement(missing) error = nil")
	}
	if patientID, ok := parsed.Dataset.GetString(tag.PatientID); !ok || patientID != "SYNTHETIC" {
		t.Fatal("fixture sanity check failed")
	}
}

func TestParseTagPathRejectsInvalidPaths(t *testing.T) {
	for _, path := range []string{"", "ContentSequence[-1]", "0040,A73G", "PatientName..PatientID"} {
		if _, err := ParseTagPath(path); err == nil {
			t.Fatalf("ParseTagPath(%q) error = nil", path)
		}
	}
}
