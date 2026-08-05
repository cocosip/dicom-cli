package inspect

import (
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
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

func TestBuildDefaultReportIncludesEncodingAndFileMeta(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	for _, elem := range []element.Element{
		element.NewString(tag.SpecificCharacterSet, vr.CS, []string{"ISO_IR 192"}),
		element.NewString(tag.Manufacturer, vr.LO, []string{"Synthetic Instruments"}),
		element.NewString(tag.ManufacturerModelName, vr.LO, []string{"Model 1"}),
		element.NewString(tag.StationName, vr.SH, []string{"STATION-1"}),
		element.NewString(tag.SoftwareVersions, vr.LO, []string{"1.2.3"}),
	} {
		if err := parsed.Dataset.AddOrUpdate(elem); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Build(fixtures.SingleFrame, parsed, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Encoding.Name != "Explicit VR Little Endian" || report.Encoding.VREncoding != "explicit" || report.Encoding.ByteOrder != "little-endian" {
		t.Fatalf("Encoding = %#v", report.Encoding)
	}
	if report.FileMeta.MediaStorageSOPClassUID != "1.2.840.10008.5.1.4.1.1.2" {
		t.Fatalf("FileMeta = %#v", report.FileMeta)
	}
	if report.Equipment.SpecificCharacterSet != "ISO_IR 192" || report.Equipment.Manufacturer != "Synthetic Instruments" || report.Equipment.Model != "Model 1" || report.Equipment.Station != "STATION-1" || report.Equipment.SoftwareVersions != "1.2.3" {
		t.Fatalf("Equipment = %#v", report.Equipment)
	}
}

func TestBuildAllIncludesFileMetaAndDatasetElements(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}

	report, err := Build(fixtures.SingleFrame, parsed, Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	var hasFileMeta, hasDataset bool
	for _, elem := range report.Elements {
		if elem.Tag == "0002,0010" && elem.Source == "file_meta" {
			hasFileMeta = true
		}
		if elem.Tag == "0010,0020" && elem.Source == "dataset" {
			hasDataset = true
		}
	}
	if !hasFileMeta || !hasDataset {
		t.Fatalf("Elements = %#v", report.Elements)
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
