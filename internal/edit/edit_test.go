package edit

import (
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

func TestApplySetsClearsAndDeletesStandardElements(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}

	err = Apply(parsed.Dataset, []Operation{
		{Kind: Set, Path: "PatientName", Value: "EDITED^PATIENT"},
		{Kind: Clear, Path: "PatientID"},
		{Kind: Delete, Path: "Modality"},
	}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	name, err := dicom.ResolveElement(parsed.Dataset, "PatientName")
	if err != nil || !strings.Contains(name.String(), "EDITED^PATIENT") {
		t.Fatalf("PatientName = %#v, %v", name, err)
	}
	patientID, err := dicom.ResolveElement(parsed.Dataset, "PatientID")
	if err != nil {
		t.Fatal(err)
	}
	if patientID.(*element.String).GetString() != "" {
		t.Fatalf("PatientID = %s, want cleared", patientID)
	}
	if _, err := dicom.ResolveElement(parsed.Dataset, "Modality"); err == nil {
		t.Fatal("Modality still exists after delete")
	}
}

func TestApplyClearPreservesNumericElementType(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(parsed.Dataset, []Operation{{Kind: Clear, Path: "Rows"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	elem, err := dicom.ResolveElement(parsed.Dataset, "Rows")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elem.(*element.UnsignedShort); !ok || elem.ValueRepresentation() != vr.US || elem.Length() != 0 {
		t.Fatalf("Rows = %#v, want an empty US element", elem)
	}
}

func TestApplyUpdatesNestedSequenceAndRequiresPrivateVR(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(parsed.Dataset, []Operation{{Kind: Set, Path: "ContentSequence[0].TextValue", Value: "changed"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	element, err := dicom.ResolveElement(parsed.Dataset, "ContentSequence[0].TextValue")
	if err != nil || !strings.Contains(element.String(), "changed") {
		t.Fatalf("TextValue = %#v, %v", element, err)
	}
	if err := Apply(parsed.Dataset, []Operation{{Kind: Set, Path: "0011,0010", Value: "private"}}, Options{}); err == nil {
		t.Fatal("private Tag without VR succeeded")
	}
	if err := Apply(parsed.Dataset, []Operation{{Kind: Set, Path: "0011,0010", Value: "private", VR: "LO"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := Apply(parsed.Dataset, []Operation{{Kind: Set, Path: "Rows", Value: "2"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	if rows := parsed.Dataset.TryGetUInt16(tag.Rows, 0); rows != 2 {
		t.Fatalf("Rows = %d, want 2", rows)
	}
}

func TestApplyGeneratesAndRemapsUIDs(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.UIDReference)
	if err != nil {
		t.Fatal(err)
	}
	original, err := dicom.ResolveElement(parsed.Dataset, "SOPInstanceUID")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(parsed.Dataset, []Operation{
		{Kind: Set, Path: "ReferencedSOPInstanceUID", Value: original.(*element.String).GetString()},
		{Kind: GenerateUID, Path: "SOPInstanceUID"},
	}, Options{RemapUIDs: true}); err != nil {
		t.Fatal(err)
	}
	sop, _ := dicom.ResolveElement(parsed.Dataset, "SOPInstanceUID")
	ref, _ := dicom.ResolveElement(parsed.Dataset, "ReferencedSOPInstanceUID")
	if !strings.Contains(sop.String(), "2.25.") || sop.(*element.String).GetString() != ref.(*element.String).GetString() {
		t.Fatalf("SOP=%s Ref=%s", sop, ref)
	}
}

func TestConvertCharacterSetUpdatesDeclarationAndRetainsText(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := ConvertCharacterSet(parsed.Dataset, "ISO_IR 192", ""); err != nil {
		t.Fatal(err)
	}
	charset, err := dicom.ResolveElement(parsed.Dataset, "SpecificCharacterSet")
	if err != nil || charset.(*element.String).GetString() != "ISO_IR 192" {
		t.Fatalf("SpecificCharacterSet = %#v, %v", charset, err)
	}
	name, err := dicom.ResolveElement(parsed.Dataset, "PatientName")
	if err != nil || name.(*element.String).GetString() != "SYNTHETIC^PATIENT" {
		t.Fatalf("PatientName = %#v, %v", name, err)
	}
}

func TestApplyGeneratesDistinctRootUIDs(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(parsed.Dataset, []Operation{{Kind: GenerateUID, Path: "StudyInstanceUID"}, {Kind: GenerateUID, Path: "SeriesInstanceUID"}}, Options{UIDRoot: "1.2.826.0.1.3680043.10.999"}); err != nil {
		t.Fatal(err)
	}
	study, _ := dicom.ResolveElement(parsed.Dataset, "StudyInstanceUID")
	series, _ := dicom.ResolveElement(parsed.Dataset, "SeriesInstanceUID")
	if !strings.HasPrefix(study.(*element.String).GetString(), "1.2.826.0.1.3680043.10.999.") || study.(*element.String).GetString() == series.(*element.String).GetString() {
		t.Fatalf("Study=%s Series=%s", study, series)
	}
}
