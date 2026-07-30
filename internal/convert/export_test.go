package convert

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
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

func TestExportFrameScalesHighBitGrayscaleForJPEG(t *testing.T) {
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
	if first > 5 || second < 250 {
		t.Fatalf("JPEG samples = %d, %d, want approximately 0, 255", first, second)
	}
}

func TestExportJSONDefaultsToPixelDataSummary(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := ExportJSON(parsed.Dataset, false)
	if err != nil {
		t.Fatal(err)
	}
	full, err := ExportJSON(parsed.Dataset, true)
	if err != nil {
		t.Fatal(err)
	}
	var summaryDocument, fullDocument map[string]any
	if err := json.Unmarshal(summary, &summaryDocument); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(full, &fullDocument); err != nil {
		t.Fatal(err)
	}
	summaryPixel := summaryDocument["7FE00010"].(map[string]any)
	fullPixel := fullDocument["7FE00010"].(map[string]any)
	if _, ok := summaryPixel["summary"]; !ok {
		t.Fatalf("summary PixelData = %#v, want summary", summaryPixel)
	}
	if _, ok := summaryPixel["InlineBinary"]; ok {
		t.Fatalf("summary PixelData = %#v, must not include bytes", summaryPixel)
	}
	if _, ok := fullPixel["InlineBinary"]; !ok {
		t.Fatalf("full PixelData = %#v, want InlineBinary", fullPixel)
	}
}
