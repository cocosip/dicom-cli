package convert

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

func TestExportFrameSelectsRequestedFrameAndPreserves16BitPNG(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.MultiFrame)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ExportFrame(parsed.Dataset, parsed.TransferSyntax, 1, PNG)
	if err != nil {
		t.Fatalf("ExportFrame() error = %v", err)
	}
	imageValue, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := imageValue.(*image.Gray16); !ok {
		t.Fatalf("PNG image type = %T, want *image.Gray16", imageValue)
	}
	if got := color.Gray16Model.Convert(imageValue.At(0, 0)).(color.Gray16).Y; got != 2 {
		t.Fatalf("frame 2 first pixel = %d, want 2", got)
	}
}

func TestTranscodeRetainsDatasetValuesAndChangesTransferSyntax(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	converted, err := Transcode(parsed.Dataset, parsed.TransferSyntax, transfer.ImplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("Transcode() error = %v", err)
	}
	if converted.InternalTransferSyntax() != transfer.ImplicitVRLittleEndian {
		t.Fatalf("transfer syntax = %v, want Implicit VR Little Endian", converted.InternalTransferSyntax())
	}
	if got, _ := converted.GetString(tag.PatientID); got != "SYNTHETIC" {
		t.Fatalf("PatientID = %q, want retained value", got)
	}
}

func TestExportFrameAppliesWindowForJPEG(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.Dataset.AddOrUpdate(element.NewOtherWord(tag.PixelData, []byte{0, 0, 0xff, 0xff})); err != nil {
		t.Fatal(err)
	}

	content, err := ExportFrame(parsed.Dataset, parsed.TransferSyntax, 0, JPEG)
	if err != nil {
		t.Fatalf("ExportFrame() error = %v", err)
	}
	imageValue, err := jpeg.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	first := color.GrayModel.Convert(imageValue.At(0, 0)).(color.Gray).Y
	second := color.GrayModel.Convert(imageValue.At(1, 0)).(color.Gray).Y
	if first < 95 || first > 110 || second < 250 {
		t.Fatalf("JPEG samples = %d, %d, want approximately 102, 255", first, second)
	}
}

func TestExportFrameAppliesCTRescaleAndWindowForJPEG(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []element.Element{
		element.NewUnsignedShort(tag.PixelRepresentation, []uint16{1}),
		element.NewString(tag.RescaleSlope, vr.DS, []string{"1"}),
		element.NewString(tag.RescaleIntercept, vr.DS, []string{"-1024"}),
		element.NewString(tag.WindowCenter, vr.DS, []string{"0", "400"}),
		element.NewString(tag.WindowWidth, vr.DS, []string{"500", "1000"}),
		element.NewOtherWord(tag.PixelData, []byte{0x00, 0x04, 0xfa, 0x04}),
	} {
		if err := parsed.Dataset.AddOrUpdate(item); err != nil {
			t.Fatal(err)
		}
	}

	content, err := ExportFrame(parsed.Dataset, parsed.TransferSyntax, 0, JPEG)
	if err != nil {
		t.Fatalf("ExportFrame() error = %v", err)
	}
	imageValue, err := jpeg.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	middle := color.GrayModel.Convert(imageValue.At(0, 0)).(color.Gray).Y
	high := color.GrayModel.Convert(imageValue.At(1, 0)).(color.Gray).Y
	if middle < 100 || middle > 155 || high < 245 {
		t.Fatalf("JPEG samples = %d, %d, want approximately 128, 255", middle, high)
	}
}

func TestExportJSONOmitsPixelData(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ExportJSON(parsed.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte(`"7FE00010"`)) {
		t.Fatalf("JSON contains PixelData: %s", content)
	}
	if _, exists := parsed.Dataset.Get(tag.PixelData); !exists {
		t.Fatal("ExportJSON removed PixelData from the source dataset")
	}
}

func TestExportXMLOmitsPixelData(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ExportXML(parsed.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte(`tag="7FE00010"`)) {
		t.Fatalf("XML contains PixelData: %s", content)
	}
	if _, exists := parsed.Dataset.Get(tag.PixelData); !exists {
		t.Fatal("ExportXML removed PixelData from the source dataset")
	}
}

func TestExportPixelDataConcatenatesRawFrames(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.MultiFrame)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ExportPixelData(parsed.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 1, 0, 2, 0, 3, 0}
	if !bytes.Equal(content, want) {
		t.Fatalf("PixelData = %v, want %v", content, want)
	}
}
