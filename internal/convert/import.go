package convert

import (
	"encoding/binary"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/cocosip/dicom-cli/internal/edit"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/uid"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

// ImportedImage is an uncompressed image accepted for Secondary Capture.
type ImportedImage struct {
	Width, Height             int
	BitsAllocated, BitsStored uint16
	SamplesPerPixel           uint16
	PhotometricInterpretation string
	PixelData                 []byte
}

// LoadImage accepts only the raster formats and pixel layouts supported by
// convert dicom: 8-bit grayscale/RGB PNG or JPEG and 16-bit grayscale PNG.
func LoadImage(path string) (ImportedImage, error) {
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
		return ImportedImage{}, fmt.Errorf("unsupported image format %q", extension)
	}
	file, err := os.Open(path)
	if err != nil {
		return ImportedImage{}, err
	}
	defer file.Close()
	imageValue, format, err := image.Decode(file)
	if err != nil {
		return ImportedImage{}, err
	}
	if format != "png" && format != "jpeg" {
		return ImportedImage{}, fmt.Errorf("unsupported decoded image format %q", format)
	}
	return importedPixels(imageValue, format)
}

func importedPixels(imageValue image.Image, format string) (ImportedImage, error) {
	bounds := imageValue.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width == 0 || height == 0 {
		return ImportedImage{}, fmt.Errorf("image dimensions must be positive")
	}
	if gray16, ok := imageValue.(*image.Gray16); ok {
		if format != "png" {
			return ImportedImage{}, fmt.Errorf("16-bit grayscale is supported only for PNG")
		}
		data := make([]byte, width*height*2)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				value := gray16.Gray16At(x, y).Y
				binary.LittleEndian.PutUint16(data[(y*width+x)*2:], value)
			}
		}
		return ImportedImage{Width: width, Height: height, BitsAllocated: 16, BitsStored: 16, SamplesPerPixel: 1, PhotometricInterpretation: "MONOCHROME2", PixelData: data}, nil
	}
	if gray, ok := imageValue.(*image.Gray); ok {
		data := make([]byte, width*height)
		for y := 0; y < height; y++ {
			copy(data[y*width:(y+1)*width], gray.Pix[y*gray.Stride:y*gray.Stride+width])
		}
		return ImportedImage{Width: width, Height: height, BitsAllocated: 8, BitsStored: 8, SamplesPerPixel: 1, PhotometricInterpretation: "MONOCHROME2", PixelData: data}, nil
	}
	data := make([]byte, width*height*3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := imageValue.At(x, y).RGBA()
			offset := (y*width + x) * 3
			data[offset], data[offset+1], data[offset+2] = uint8(r>>8), uint8(g>>8), uint8(b>>8)
		}
	}
	return ImportedImage{Width: width, Height: height, BitsAllocated: 8, BitsStored: 8, SamplesPerPixel: 3, PhotometricInterpretation: "RGB", PixelData: data}, nil
}

// SecondaryCaptureOptions contains values not inferable from the image.
type SecondaryCaptureOptions struct {
	PatientName, StudyUID, SeriesUID, SOPInstanceUID string
}

// NewSecondaryCapture creates an Explicit VR Little Endian-compatible
// Secondary Capture dataset from a supported imported image.
func NewSecondaryCapture(imageValue ImportedImage, options SecondaryCaptureOptions) (*dataset.Dataset, error) {
	if options.PatientName == "" {
		return nil, fmt.Errorf("PatientName is required")
	}
	if options.StudyUID == "" {
		options.StudyUID = uid.GenerateDerivedFromUUID().UID()
	}
	if options.SeriesUID == "" {
		options.SeriesUID = uid.GenerateDerivedFromUUID().UID()
	}
	if options.SOPInstanceUID == "" {
		options.SOPInstanceUID = uid.GenerateDerivedFromUUID().UID()
	}
	ds := dataset.New()
	elements := []element.Element{
		element.NewString(tag.PatientName, vr.PN, []string{options.PatientName}),
		element.NewString(tag.StudyInstanceUID, vr.UI, []string{options.StudyUID}),
		element.NewString(tag.SeriesInstanceUID, vr.UI, []string{options.SeriesUID}),
		element.NewString(tag.SOPClassUID, vr.UI, []string{uid.SecondaryCaptureImageStorage.UID()}),
		element.NewString(tag.SOPInstanceUID, vr.UI, []string{options.SOPInstanceUID}),
		element.NewString(tag.Modality, vr.CS, []string{"OT"}),
		element.NewString(tag.ConversionType, vr.CS, []string{"WSD"}),
		element.NewUnsignedShort(tag.Rows, []uint16{uint16(imageValue.Height)}),
		element.NewUnsignedShort(tag.Columns, []uint16{uint16(imageValue.Width)}),
		element.NewUnsignedShort(tag.SamplesPerPixel, []uint16{imageValue.SamplesPerPixel}),
		element.NewString(tag.PhotometricInterpretation, vr.CS, []string{imageValue.PhotometricInterpretation}),
		element.NewUnsignedShort(tag.BitsAllocated, []uint16{imageValue.BitsAllocated}),
		element.NewUnsignedShort(tag.BitsStored, []uint16{imageValue.BitsStored}),
		element.NewUnsignedShort(tag.HighBit, []uint16{imageValue.BitsStored - 1}),
		element.NewUnsignedShort(tag.PixelRepresentation, []uint16{0}),
	}
	if imageValue.SamplesPerPixel == 3 {
		elements = append(elements, element.NewUnsignedShort(tag.PlanarConfiguration, []uint16{0}))
	}
	if imageValue.BitsAllocated == 16 {
		elements = append(elements, element.NewOtherWord(tag.PixelData, imageValue.PixelData))
	} else {
		elements = append(elements, element.NewOtherByte(tag.PixelData, imageValue.PixelData))
	}
	for _, item := range elements {
		if err := ds.Add(item); err != nil {
			return nil, err
		}
	}
	return ds, nil
}

// ApplyMetadata merges metadata sources in order. Later sources override an
// earlier value for the same DICOM tag path.
func ApplyMetadata(ds *dataset.Dataset, sources ...map[string]string) error {
	for _, source := range sources {
		operations := make([]edit.Operation, 0, len(source))
		for path, value := range source {
			operations = append(operations, edit.Operation{Kind: edit.Set, Path: path, Value: value})
		}
		if err := edit.Apply(ds, operations, edit.Options{}); err != nil {
			return err
		}
	}
	return nil
}
