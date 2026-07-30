// Package inspect projects one parsed DICOM file into a command report.
package inspect

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

type Options struct {
	All  bool
	Tags []string
}

type Report struct {
	File     FileSummary     `json:"file"`
	Patient  PatientSummary  `json:"patient"`
	Study    StudySummary    `json:"study"`
	Series   SeriesSummary   `json:"series"`
	Instance InstanceSummary `json:"instance"`
	Pixel    PixelSummary    `json:"pixel"`
	Elements []ElementReport `json:"elements,omitempty"`
	Tags     []ElementReport `json:"tags,omitempty"`
}

type FileSummary struct {
	Path           string `json:"path"`
	TransferSyntax string `json:"transfer_syntax"`
}
type PatientSummary struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	BirthDate string `json:"birth_date"`
	Sex       string `json:"sex"`
}
type StudySummary struct {
	InstanceUID        string `json:"instance_uid"`
	Modality           string `json:"modality"`
	Date               string `json:"date"`
	Time               string `json:"time"`
	AccessionNumber    string `json:"accession_number"`
	Description        string `json:"description"`
	ReferringPhysician string `json:"referring_physician"`
}
type SeriesSummary struct {
	InstanceUID  string `json:"instance_uid"`
	Number       string `json:"number"`
	Description  string `json:"description"`
	BodyPart     string `json:"body_part"`
	Laterality   string `json:"laterality"`
	ProtocolName string `json:"protocol_name"`
}
type InstanceSummary struct {
	SOPClassUID          string `json:"sop_class_uid"`
	SOPInstanceUID       string `json:"sop_instance_uid"`
	Number               string `json:"number"`
	ImagePosition        string `json:"image_position"`
	ImageOrientation     string `json:"image_orientation"`
	SliceThickness       string `json:"slice_thickness"`
	SpacingBetweenSlices string `json:"spacing_between_slices"`
}
type PixelSummary struct {
	Rows                      int    `json:"rows"`
	Columns                   int    `json:"columns"`
	Frames                    int    `json:"frames"`
	Bytes                     int    `json:"bytes"`
	SamplesPerPixel           int    `json:"samples_per_pixel"`
	PhotometricInterpretation string `json:"photometric_interpretation"`
	BitsAllocated             int    `json:"bits_allocated"`
	BitsStored                int    `json:"bits_stored"`
	HighBit                   int    `json:"high_bit"`
	PixelRepresentation       int    `json:"pixel_representation"`
	PixelSpacing              string `json:"pixel_spacing"`
	WindowCenter              string `json:"window_center"`
	WindowWidth               string `json:"window_width"`
}
type ElementReport struct {
	Path  string `json:"path"`
	Tag   string `json:"tag"`
	VR    string `json:"vr"`
	Value string `json:"value"`
}

func Build(path string, parsed *parser.ParseResult, options Options) (Report, error) {
	if parsed == nil || parsed.Dataset == nil {
		return Report{}, fmt.Errorf("parsed dataset is required")
	}
	report := Report{
		File: FileSummary{Path: filepath.Clean(path)},
		Patient: PatientSummary{
			Name:      stringValue(parsed.Dataset, tag.PatientName),
			ID:        stringValue(parsed.Dataset, tag.PatientID),
			BirthDate: stringValue(parsed.Dataset, tag.PatientBirthDate),
			Sex:       stringValue(parsed.Dataset, tag.PatientSex),
		},
		Study: StudySummary{
			InstanceUID:        stringValue(parsed.Dataset, tag.StudyInstanceUID),
			Modality:           stringValue(parsed.Dataset, tag.Modality),
			Date:               stringValue(parsed.Dataset, tag.StudyDate),
			Time:               stringValue(parsed.Dataset, tag.StudyTime),
			AccessionNumber:    stringValue(parsed.Dataset, tag.AccessionNumber),
			Description:        stringValue(parsed.Dataset, tag.StudyDescription),
			ReferringPhysician: stringValue(parsed.Dataset, tag.ReferringPhysicianName),
		},
		Series: SeriesSummary{
			InstanceUID:  stringValue(parsed.Dataset, tag.SeriesInstanceUID),
			Number:       stringValue(parsed.Dataset, tag.SeriesNumber),
			Description:  stringValue(parsed.Dataset, tag.SeriesDescription),
			BodyPart:     stringValue(parsed.Dataset, tag.BodyPartExamined),
			Laterality:   stringValue(parsed.Dataset, tag.Laterality),
			ProtocolName: stringValue(parsed.Dataset, tag.ProtocolName),
		},
		Instance: InstanceSummary{
			SOPClassUID:          stringValue(parsed.Dataset, tag.SOPClassUID),
			SOPInstanceUID:       stringValue(parsed.Dataset, tag.SOPInstanceUID),
			Number:               stringValue(parsed.Dataset, tag.InstanceNumber),
			ImagePosition:        stringValue(parsed.Dataset, tag.ImagePositionPatient),
			ImageOrientation:     stringValue(parsed.Dataset, tag.ImageOrientationPatient),
			SliceThickness:       stringValue(parsed.Dataset, tag.SliceThickness),
			SpacingBetweenSlices: stringValue(parsed.Dataset, tag.SpacingBetweenSlices),
		},
		Pixel: PixelSummary{
			Rows:                      int(uint16Value(parsed.Dataset, tag.Rows)),
			Columns:                   int(uint16Value(parsed.Dataset, tag.Columns)),
			Frames:                    frameCount(parsed.Dataset),
			Bytes:                     elementLength(parsed.Dataset, tag.PixelData),
			SamplesPerPixel:           int(uint16Value(parsed.Dataset, tag.SamplesPerPixel)),
			PhotometricInterpretation: stringValue(parsed.Dataset, tag.PhotometricInterpretation),
			BitsAllocated:             int(uint16Value(parsed.Dataset, tag.BitsAllocated)),
			BitsStored:                int(uint16Value(parsed.Dataset, tag.BitsStored)),
			HighBit:                   int(uint16Value(parsed.Dataset, tag.HighBit)),
			PixelRepresentation:       int(uint16Value(parsed.Dataset, tag.PixelRepresentation)),
			PixelSpacing:              stringValue(parsed.Dataset, tag.PixelSpacing),
			WindowCenter:              stringValue(parsed.Dataset, tag.WindowCenter),
			WindowWidth:               stringValue(parsed.Dataset, tag.WindowWidth),
		},
	}
	if parsed.TransferSyntax != nil {
		report.File.TransferSyntax = parsed.TransferSyntax.UID().String()
	}
	if options.All {
		for _, elem := range parsed.Dataset.Elements() {
			report.Elements = append(report.Elements, describe(elem.Tag().Format("g"), elem))
		}
	}
	for _, path := range options.Tags {
		elem, err := dicom.ResolveElement(parsed.Dataset, path)
		if err != nil {
			return Report{}, err
		}
		report.Tags = append(report.Tags, describe(path, elem))
	}
	return report, nil
}

func describe(path string, elem element.Element) ElementReport {
	value := elem.String()
	if stringElement, ok := elem.(*element.String); ok {
		value = stringElement.GetString()
	}
	return ElementReport{Path: path, Tag: elem.Tag().Format("g"), VR: elem.ValueRepresentation().Code(), Value: value}
}

func stringValue(ds *dataset.Dataset, t *tag.Tag) string { return ds.TryGetString(t) }
func uint16Value(ds *dataset.Dataset, t *tag.Tag) uint16 { return ds.TryGetUInt16(t, 0) }
func elementLength(ds *dataset.Dataset, t *tag.Tag) int {
	elem, ok := ds.Get(t)
	if !ok {
		return 0
	}
	return int(elem.Length())
}
func frameCount(ds *dataset.Dataset) int {
	value := stringValue(ds, tag.NumberOfFrames)
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		return 1
	}
	return count
}
