// Package convert implements local DICOM and image conversion operations.
package convert

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/serialization"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// ImageFormat is an image format supported by DICOM frame export.
type ImageFormat string

const (
	PNG  ImageFormat = "png"
	JPEG ImageFormat = "jpeg"
)

// ExportFrame encodes a zero-indexed DICOM frame without applying display
// transforms such as windowing, LUTs, or rescale slope/intercept.
func ExportFrame(ds *dataset.Dataset, syntax *transfer.Syntax, frame int, format ImageFormat) ([]byte, error) {
	imageValue, err := frameImage(ds, syntax, frame, format == JPEG)
	if err != nil {
		return nil, err
	}
	var content bytes.Buffer
	switch format {
	case PNG:
		err = png.Encode(&content, imageValue)
	case JPEG:
		err = jpeg.Encode(&content, imageValue, &jpeg.Options{Quality: 95})
	default:
		return nil, fmt.Errorf("unsupported image format %q", format)
	}
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// ExportJSON produces DICOM JSON. PixelData is summarized unless callers
// explicitly request the serialized inline binary representation.
func ExportJSON(ds *dataset.Dataset, includePixelData bool) ([]byte, error) {
	content, err := serialization.ToJSON(ds, serialization.WithIndent("  "))
	if err != nil || includePixelData {
		return content, err
	}
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	if pixel, ok := document["7FE00010"].(map[string]any); ok {
		document["7FE00010"] = map[string]any{
			"vr": pixel["vr"],
			"summary": map[string]any{
				"present": true,
			},
		}
	}
	return json.MarshalIndent(document, "", "  ")
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
