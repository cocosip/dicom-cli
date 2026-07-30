package anonymize

import (
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

func TestAnonymizeEngineAppliesBasicProfile(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(Options{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Anonymize(parsed.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	patientName, _ := result.Dataset.GetString(tag.PatientName)
	if patientName == "SYNTHETIC^PATIENT" {
		t.Fatalf("PatientName was not anonymized: %q", patientName)
	}
	studyUID, _ := result.Dataset.GetString(tag.StudyInstanceUID)
	if studyUID == "1.2.826.0.1.3680043.10.5432" {
		t.Fatalf("StudyInstanceUID was not remapped: %q", studyUID)
	}
	if source, _ := parsed.Dataset.GetString(tag.PatientName); source != "SYNTHETIC^PATIENT" {
		t.Fatalf("source dataset was modified: %q", source)
	}
}

func TestAnonymizeEngineRetainsUIDsWhenSelected(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(Options{ProfileOptions: []string{"retain-uids"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Anonymize(parsed.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := result.Dataset.GetString(tag.StudyInstanceUID); got != "1.2.826.0.1.3680043.10.5432" {
		t.Fatalf("StudyInstanceUID = %q, want original UID", got)
	}
}

func TestAnonymizeEngineRejectsUnknownProfileOption(t *testing.T) {
	_, err := NewEngine(Options{ProfileOptions: []string{"not-an-option"}})
	if err == nil || !strings.Contains(err.Error(), "not-an-option") {
		t.Fatalf("NewEngine() error = %v, want unknown option", err)
	}
}

func TestAnonymizeEngineAppliesExternalRulesAfterBasicProfile(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(Options{Rules: []rules.AnonymizeRule{
		{Path: "PatientName", Action: "replace", Value: stringPtr("ANON^SUBJECT")},
		{Path: "PatientID", Action: "clear"},
		{Path: "Modality", Action: "delete"},
		{Path: "SeriesInstanceUID", Action: "remap_uid"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Anonymize(parsed.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := result.Dataset.GetString(tag.PatientName); got != "ANON^SUBJECT" {
		t.Fatalf("PatientName = %q, want fixed replacement", got)
	}
	if got, _ := result.Dataset.GetString(tag.PatientID); got != "" {
		t.Fatalf("PatientID = %q, want empty", got)
	}
	if _, ok := result.Dataset.Get(tag.Modality); ok {
		t.Fatal("Modality still present after delete")
	}
	if got, _ := result.Dataset.GetString(tag.SeriesInstanceUID); got == "1.2.826.0.1.3680043.10.5432.1" {
		t.Fatal("SeriesInstanceUID was not remapped")
	}
}

func TestAnonymizeEngineSharesUIDMappingAcrossDatasetsAndReferences(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	second, err := parser.ParseFile(fixtures.UIDReference)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(Options{})
	if err != nil {
		t.Fatal(err)
	}
	firstResult, err := engine.Anonymize(first.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	secondResult, err := engine.Anonymize(second.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	firstSOP, _ := firstResult.Dataset.GetString(tag.SOPInstanceUID)
	referencedSOP, _ := secondResult.Dataset.GetString(tag.ReferencedSOPInstanceUID)
	if firstSOP != referencedSOP {
		t.Fatalf("shared UID mapping mismatch: SOP=%q reference=%q", firstSOP, referencedSOP)
	}
	for _, item := range []struct {
		name string
		tag  *tag.Tag
	}{{"StudyInstanceUID", tag.StudyInstanceUID}, {"SeriesInstanceUID", tag.SeriesInstanceUID}} {
		firstValue, _ := firstResult.Dataset.GetString(item.tag)
		secondValue, _ := secondResult.Dataset.GetString(item.tag)
		if firstValue != secondValue {
			t.Fatalf("shared UID mapping mismatch for %s: first=%q second=%q", item.name, firstValue, secondValue)
		}
	}
}

func TestAnonymizeEngineDetailedResultContainsChangesAndUIDMappings(t *testing.T) {
	dataset := testDataset(t)
	engine, err := NewEngine(Options{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Anonymize(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changes) == 0 {
		t.Fatal("detailed result has no changes")
	}
	if len(result.UIDMappings) == 0 {
		t.Fatal("detailed result has no UID mappings")
	}
}

func TestAnonymizeEngineRetainsPrivateTagsByDefault(t *testing.T) {
	dataset := testDataset(t)
	privateTag := tag.New(0x0011, 0x0010)
	if err := dataset.AddOrUpdate(element.NewString(privateTag, vr.LO, []string{"private value"})); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(Options{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Anonymize(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := result.Dataset.GetString(privateTag); !ok || got != "private value" {
		t.Fatalf("private Tag = %q, exists=%v; want retained source value", got, ok)
	}
}

func testDataset(t *testing.T) *dataset.Dataset {
	t.Helper()
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.Dataset.AddOrUpdate(element.NewString(tag.PatientComments, vr.LT, []string{"sensitive"})); err != nil {
		t.Fatal(err)
	}
	return parsed.Dataset
}

func stringPtr(value string) *string { return &value }
