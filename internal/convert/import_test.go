package convert

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

func TestLoadImageAccepts16BitGrayPNGAndRejectsUnsupportedExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gray16.png")
	imageValue := image.NewGray16(image.Rect(0, 0, 2, 1))
	imageValue.SetGray16(0, 0, color.Gray16{Y: 1})
	imageValue.SetGray16(1, 0, color.Gray16{Y: 65535})
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, imageValue); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadImage(path)
	if err != nil {
		t.Fatalf("LoadImage() error = %v", err)
	}
	if loaded.BitsAllocated != 16 || loaded.SamplesPerPixel != 1 || len(loaded.PixelData) != 4 {
		t.Fatalf("loaded image = %+v", loaded)
	}
	if _, err := LoadImage(filepath.Join(t.TempDir(), "image.bmp")); err == nil {
		t.Fatal("LoadImage(BMP) succeeded")
	}
}

func TestNewSecondaryCaptureRequiresPatientNameAndBuildsImageDataset(t *testing.T) {
	imageValue := ImportedImage{Width: 2, Height: 1, BitsAllocated: 8, BitsStored: 8, SamplesPerPixel: 1, PhotometricInterpretation: "MONOCHROME2", PixelData: []byte{0, 255}}
	if _, err := NewSecondaryCapture(imageValue, SecondaryCaptureOptions{}); err == nil {
		t.Fatal("NewSecondaryCapture() succeeded without required PatientName")
	}
	dataset, err := NewSecondaryCapture(imageValue, SecondaryCaptureOptions{PatientName: "SYNTHETIC^PATIENT"})
	if err != nil {
		t.Fatalf("NewSecondaryCapture() error = %v", err)
	}
	if got, _ := dataset.GetString(tag.PatientName); got != "SYNTHETIC^PATIENT" {
		t.Fatalf("PatientName = %q", got)
	}
	if got, _ := dataset.GetString(tag.SOPClassUID); got != "1.2.840.10008.5.1.4.1.1.7" {
		t.Fatalf("SOPClassUID = %q", got)
	}
	if got, _ := dataset.GetString(tag.StudyInstanceUID); got == "" {
		t.Fatal("StudyInstanceUID is empty")
	}
}

func TestApplyMetadataUsesLaterSourcesAsOverrides(t *testing.T) {
	imageValue := ImportedImage{Width: 1, Height: 1, BitsAllocated: 8, BitsStored: 8, SamplesPerPixel: 1, PhotometricInterpretation: "MONOCHROME2", PixelData: []byte{0}}
	dataset, err := NewSecondaryCapture(imageValue, SecondaryCaptureOptions{PatientName: "SYNTHETIC^PATIENT"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyMetadata(dataset, map[string]string{"PatientID": "template", "PatientName": "TEMPLATE^PATIENT"}, map[string]string{"PatientID": "reference"}, map[string]string{"PatientName": "CLI^PATIENT"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := dataset.GetString(tag.PatientID); got != "reference" {
		t.Fatalf("PatientID = %q, want reference", got)
	}
	if got, _ := dataset.GetString(tag.PatientName); got != "CLI^PATIENT" {
		t.Fatalf("PatientName = %q, want CLI", got)
	}
}
