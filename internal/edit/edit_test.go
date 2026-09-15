package edit

import (
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/charset"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/io/buffer"
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
	name, ok := parsed.Dataset.GetString(tag.PatientName)
	if !ok || name != "EDITED^PATIENT" {
		t.Fatalf("PatientName = %q, found=%t", name, ok)
	}
	patientID, ok := parsed.Dataset.GetString(tag.PatientID)
	if !ok || patientID != "" {
		t.Fatalf("PatientID = %q, found=%t, want cleared", patientID, ok)
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

func TestApplySetUsesDatasetCharacterSetForNewTextElement(t *testing.T) {
	ds := dataset.New()
	if err := ds.SetSpecificCharacterSet("ISO_IR 192"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ds, []Operation{{Kind: Set, Path: "InstitutionName", Value: "示例医院"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	value, err := ds.GetStringWithError(tag.InstitutionName)
	if err != nil {
		t.Fatal(err)
	}
	if value != "示例医院" {
		t.Fatalf("InstitutionName = %q, want 示例医院", value)
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
	original, ok := parsed.Dataset.GetString(tag.SOPInstanceUID)
	if !ok {
		t.Fatal("SOPInstanceUID is missing")
	}
	if err := Apply(parsed.Dataset, []Operation{
		{Kind: Set, Path: "ReferencedSOPInstanceUID", Value: original},
		{Kind: GenerateUID, Path: "SOPInstanceUID"},
	}, Options{RemapUIDs: true}); err != nil {
		t.Fatal(err)
	}
	sop, _ := parsed.Dataset.GetString(tag.SOPInstanceUID)
	ref, _ := parsed.Dataset.GetString(tag.ReferencedSOPInstanceUID)
	if !strings.Contains(sop, "2.25.") || sop != ref {
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
	characterSet, ok := parsed.Dataset.GetString(tag.SpecificCharacterSet)
	if !ok || characterSet != "ISO_IR 192" {
		t.Fatalf("SpecificCharacterSet = %q, found=%t", characterSet, ok)
	}
	name, ok := parsed.Dataset.GetString(tag.PatientName)
	if !ok || name != "SYNTHETIC^PATIENT" {
		t.Fatalf("PatientName = %q, found=%t", name, ok)
	}
}

func TestConvertCharacterSetReencodesTypedPersonName(t *testing.T) {
	ds := dataset.New()
	if err := ds.SetSpecificCharacterSet("ISO_IR 192"); err != nil {
		t.Fatal(err)
	}
	if err := ds.AddValue(tag.PatientName, "张三"); err != nil {
		t.Fatal(err)
	}
	if err := ConvertCharacterSet(ds, "GB18030", ""); err != nil {
		t.Fatal(err)
	}
	elem, ok := ds.Get(tag.PatientName)
	if !ok {
		t.Fatal("PatientName is missing")
	}
	if _, ok := elem.(*element.PersonName); !ok {
		t.Fatalf("PatientName type = %T, want *element.PersonName", elem)
	}
	decoded, err := charset.DecodeString(elem.Buffer().Data(), charset.GetEncodings([]string{"GB18030"}))
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "张三" {
		t.Fatalf("decoded PatientName = %q, want 张三", decoded)
	}
}

func TestConvertCharacterSetInputOverrideDecodesRawText(t *testing.T) {
	encoded, err := charset.EncodeString("张三", charset.GetEncodings([]string{"GB18030"}))
	if err != nil {
		t.Fatal(err)
	}
	ds := dataset.New()
	ds.SetAutoValidate(false)
	patientName, err := element.NewElementFromBuffer(tag.PatientName, vr.PN, buffer.NewMemory(encoded), charset.GetEncodings(nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := ds.Add(patientName); err != nil {
		t.Fatal(err)
	}
	if err := ConvertCharacterSet(ds, "ISO_IR 192", "GB18030"); err != nil {
		t.Fatal(err)
	}
	value, err := ds.GetStringWithError(tag.PatientName)
	if err != nil {
		t.Fatal(err)
	}
	if value != "张三" {
		t.Fatalf("PatientName = %q, want 张三", value)
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
	study, _ := parsed.Dataset.GetString(tag.StudyInstanceUID)
	series, _ := parsed.Dataset.GetString(tag.SeriesInstanceUID)
	if !strings.HasPrefix(study, "1.2.826.0.1.3680043.10.999.") || study == series {
		t.Fatalf("Study=%s Series=%s", study, series)
	}
}
