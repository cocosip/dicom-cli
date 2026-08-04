// Package convert implements local DICOM and image conversion operations.
package convert

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strconv"
	"strings"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/serialization"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/render"
)

// ImageFormat is an image format supported by DICOM frame export.
type ImageFormat string

const (
	PNG  ImageFormat = "png"
	JPEG ImageFormat = "jpeg"
)

// ExportFrame encodes a zero-indexed DICOM frame. JPEG grayscale output uses
// the DICOM rescale and window settings so it is suitable for display.
func ExportFrame(ds *dataset.Dataset, syntax *transfer.Syntax, frame int, format ImageFormat) ([]byte, error) {
	if format == JPEG {
		return exportJPEG(ds, syntax, frame)
	}
	imageValue, err := frameImage(ds, syntax, frame, format == JPEG)
	if err != nil {
		return nil, err
	}
	var content bytes.Buffer
	switch format {
	case PNG:
		err = png.Encode(&content, imageValue)
	default:
		return nil, fmt.Errorf("unsupported image format %q", format)
	}
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func exportJPEG(ds *dataset.Dataset, syntax *transfer.Syntax, frame int) ([]byte, error) {
	pixelData, err := imaging.CreatePixelData(ds)
	if err != nil {
		return nil, err
	}
	if frame < 0 || frame >= pixelData.FrameCount() {
		return nil, fmt.Errorf("frame %d is outside [0, %d)", frame, pixelData.FrameCount())
	}
	if pixelData.Info.SamplesPerPixel != 1 {
		imageValue, err := frameImage(ds, syntax, frame, false)
		if err != nil {
			return nil, err
		}
		var content bytes.Buffer
		if err := jpeg.Encode(&content, imageValue, &jpeg.Options{Quality: 95}); err != nil {
			return nil, err
		}
		return content.Bytes(), nil
	}

	raw, err := decodedFrame(ds, syntax, pixelData, frame)
	if err != nil {
		return nil, err
	}
	slope, intercept, center, width, err := grayscaleDisplaySettings(ds, pixelData.Info)
	if err != nil {
		return nil, err
	}
	minInput, maxInput := grayscaleRange(pixelData.Info.BitsStored, pixelData.Info.PixelRepresentation == imaging.SignedPixels)
	pipeline := render.NewGrayscalePipeline(slope, intercept, center, width, minInput, maxInput, false)
	exporter := render.NewImageExporter(pipeline)
	photometric := "MONOCHROME2"
	if pixelData.Info.PhotometricInterpretation != nil {
		photometric = pixelData.Info.PhotometricInterpretation.Value
	}
	var content bytes.Buffer
	err = exporter.ExportGrayscale(
		&content,
		raw,
		int(pixelData.Info.Width),
		int(pixelData.Info.Height),
		int(pixelData.Info.BitsAllocated),
		int(pixelData.Info.BitsStored),
		pixelData.Info.PixelRepresentation == imaging.SignedPixels,
		photometric,
		&render.ExportOptions{Format: render.FormatJPEG, JPEGQuality: 95},
	)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func grayscaleDisplaySettings(ds *dataset.Dataset, info *imaging.PixelDataInfo) (slope, intercept, center, width float64, err error) {
	slope, err = dicomDecimal(ds, tag.RescaleSlope, 1)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	intercept, err = dicomDecimal(ds, tag.RescaleIntercept, 0)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	minInput, maxInput := grayscaleRange(info.BitsStored, info.PixelRepresentation == imaging.SignedPixels)
	minOutput := minInput*slope + intercept
	maxOutput := maxInput*slope + intercept
	if minOutput > maxOutput {
		minOutput, maxOutput = maxOutput, minOutput
	}
	center = (minOutput + maxOutput) / 2
	width = maxOutput - minOutput + 1
	center, err = dicomDecimal(ds, tag.WindowCenter, center)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	width, err = dicomDecimal(ds, tag.WindowWidth, width)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if width < 1 {
		return 0, 0, 0, 0, fmt.Errorf("WindowWidth must be at least 1")
	}
	return slope, intercept, center, width, nil
}

func grayscaleRange(bitsStored uint16, signed bool) (float64, float64) {
	if bitsStored == 0 || bitsStored > 16 {
		bitsStored = 16
	}
	if signed {
		maximum := float64((uint32(1) << (bitsStored - 1)) - 1)
		return -maximum - 1, maximum
	}
	return 0, float64((uint32(1) << bitsStored) - 1)
}

func dicomDecimal(ds *dataset.Dataset, field *tag.Tag, fallback float64) (float64, error) {
	value, ok := ds.GetString(field)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	value = strings.TrimSpace(strings.SplitN(value, "\\", 2)[0])
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("invalid %s value %q", field, value)
	}
	return parsed, nil
}

// ExportJSON produces DICOM JSON metadata without PixelData.
func ExportJSON(ds *dataset.Dataset) ([]byte, error) {
	return serialization.ToJSON(metadataDataset(ds), serialization.WithIndent("  "))
}

// ExportXML produces Native DICOM Model XML metadata without PixelData.
func ExportXML(ds *dataset.Dataset) ([]byte, error) {
	return serialization.ToXML(metadataDataset(ds))
}

// ExportPixelData concatenates the stored payload of each frame without
// decoding or transcoding it.
func ExportPixelData(ds *dataset.Dataset) ([]byte, error) {
	pixelData, err := imaging.CreatePixelData(ds)
	if err != nil {
		return nil, err
	}
	var content bytes.Buffer
	for frame := 0; frame < pixelData.FrameCount(); frame++ {
		data, err := pixelData.GetFrame(frame)
		if err != nil {
			return nil, err
		}
		if _, err := content.Write(data); err != nil {
			return nil, err
		}
	}
	return content.Bytes(), nil
}

func metadataDataset(ds *dataset.Dataset) *dataset.Dataset {
	clone := ds.Clone()
	clone.Remove(tag.PixelData)
	return clone
}

// FrameCount returns the number of frames in a DICOM dataset.
func FrameCount(ds *dataset.Dataset) (int, error) {
	pixelData, err := imaging.CreatePixelData(ds)
	if err != nil {
		return 0, err
	}
	return pixelData.FrameCount(), nil
}

// Transcode converts dataset pixel data to target while preserving unrelated
// dataset elements. The target syntax must be available in the process codec
// registry.
func Transcode(ds *dataset.Dataset, source, target *transfer.Syntax) (*dataset.Dataset, error) {
	if source == nil || target == nil {
		return nil, fmt.Errorf("source and target transfer syntaxes are required")
	}
	transcoder, err := codec.GetDefaultManager().CreateTranscoder(source, target)
	if err != nil {
		return nil, err
	}
	return transcoder.Transcode(ds)
}

func frameImage(ds *dataset.Dataset, syntax *transfer.Syntax, frame int, jpegTarget bool) (image.Image, error) {
	pixelData, err := imaging.CreatePixelData(ds)
	if err != nil {
		return nil, err
	}
	if frame < 0 || frame >= pixelData.FrameCount() {
		return nil, fmt.Errorf("frame %d is outside [0, %d)", frame, pixelData.FrameCount())
	}
	raw, err := decodedFrame(ds, syntax, pixelData, frame)
	if err != nil {
		return nil, err
	}
	info := pixelData.Info
	if info.SamplesPerPixel == 1 {
		return grayscaleImage(raw, int(info.Width), int(info.Height), info.BitsAllocated, info.BitsStored, jpegTarget)
	}
	if info.SamplesPerPixel == 3 && info.BitsAllocated == 8 {
		if info.PlanarConfiguration == imaging.PlanarPlanar {
			raw, err = imaging.ConvertPlanarToInterleavedGeneric(raw, 3, 1)
			if err != nil {
				return nil, err
			}
		}
		return rgbImage(raw, int(info.Width), int(info.Height))
	}
	return nil, fmt.Errorf("unsupported pixel layout: samples=%d bits=%d", info.SamplesPerPixel, info.BitsAllocated)
}

func decodedFrame(ds *dataset.Dataset, syntax *transfer.Syntax, pixelData *imaging.DicomPixelData, frame int) ([]byte, error) {
	if syntax == nil || !syntax.IsEncapsulated() {
		return pixelData.GetFrame(frame)
	}
	transcoder, err := codec.GetDefaultManager().CreateTranscoder(syntax, transfer.ExplicitVRLittleEndian)
	if err != nil {
		return nil, err
	}
	return transcoder.DecodeFrame(ds, frame)
}

func grayscaleImage(raw []byte, width, height int, bitsAllocated, bitsStored uint16, jpegTarget bool) (image.Image, error) {
	pixels := width * height
	if bitsAllocated == 8 {
		if len(raw) < pixels {
			return nil, fmt.Errorf("pixel data is shorter than image dimensions")
		}
		if jpegTarget || bitsStored <= 8 {
			result := image.NewGray(image.Rect(0, 0, width, height))
			copy(result.Pix, raw[:pixels])
			return result, nil
		}
	}
	if bitsAllocated != 16 || len(raw) < pixels*2 {
		return nil, fmt.Errorf("unsupported grayscale bits allocated %d", bitsAllocated)
	}
	if jpegTarget {
		result := image.NewGray(image.Rect(0, 0, width, height))
		maxValue := uint32((uint64(1) << bitsStored) - 1)
		for index := range result.Pix {
			value := uint32(binary.LittleEndian.Uint16(raw[index*2:])) & maxValue
			result.Pix[index] = uint8(value * 255 / maxValue)
		}
		return result, nil
	}
	result := image.NewGray16(image.Rect(0, 0, width, height))
	for index := 0; index < pixels; index++ {
		binary.BigEndian.PutUint16(result.Pix[index*2:], binary.LittleEndian.Uint16(raw[index*2:]))
	}
	return result, nil
}

func rgbImage(raw []byte, width, height int) (image.Image, error) {
	pixels := width * height
	if len(raw) < pixels*3 {
		return nil, fmt.Errorf("pixel data is shorter than RGB image dimensions")
	}
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	for index := 0; index < pixels; index++ {
		offset := index * 3
		result.SetRGBA(index%width, index/width, color.RGBA{R: raw[offset], G: raw[offset+1], B: raw[offset+2], A: 255})
	}
	return result, nil
}
