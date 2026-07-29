package testutil

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

func TestCreateDICOMFixturesProducesReusableSyntheticCases(t *testing.T) {
	fixtures, err := CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatalf("CreateDICOMFixtures() error = %v", err)
	}

	for name, path := range map[string]string{
		"single frame":  fixtures.SingleFrame,
		"multi frame":   fixtures.MultiFrame,
		"sequence":      fixtures.Sequence,
		"uid reference": fixtures.UIDReference,
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := parser.ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile(%q): %v", path, err)
			}
			if patientID, ok := parsed.Dataset.GetString(tag.PatientID); !ok || patientID != "SYNTHETIC" {
				t.Fatalf("PatientID = %q, %t; want SYNTHETIC, true", patientID, ok)
			}
		})
	}

	multiFrame, err := parser.ParseFile(fixtures.MultiFrame)
	if err != nil {
		t.Fatal(err)
	}
	if frames, ok := multiFrame.Dataset.GetString(tag.NumberOfFrames); !ok || frames != "2" {
		t.Fatalf("NumberOfFrames = %q, %t; want 2, true", frames, ok)
	}

	sequence, err := parser.ParseFile(fixtures.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	element, ok := sequence.Dataset.Get(tag.ContentSequence)
	if !ok {
		t.Fatal("ContentSequence is missing")
	}
	if value, ok := element.(*dataset.Sequence); !ok || value.Count() != 1 {
		t.Fatalf("ContentSequence = %#v, want one-item sequence", element)
	}

	if _, err := parser.ParseFile(fixtures.Corrupt); err == nil {
		t.Fatal("ParseFile(corrupt) error = nil, want parse error")
	}
}
