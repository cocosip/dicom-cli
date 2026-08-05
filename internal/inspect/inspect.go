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
	File      FileSummary      `json:"file"`
	Encoding  EncodingSummary  `json:"encoding"`
	FileMeta  FileMetaSummary  `json:"file_meta"`
	Equipment EquipmentSummary `json:"equipment"`
	Patient   PatientSummary   `json:"patient"`
	Study     StudySummary     `json:"study"`
	Series    SeriesSummary    `json:"series"`
	Instance  InstanceSummary  `json:"instance"`
	Pixel     PixelSummary     `json:"pixel"`
	Elements  []ElementReport  `json:"elements,omitempty"`
	Tags      []ElementReport  `json:"tags,omitempty"`
}

type FileSummary struct {
	Path           string `json:"path"`
	TransferSyntax string `json:"transfer_syntax"`
}
type EncodingSummary struct {
	UID                    string `json:"uid"`
	Name                   string `json:"name"`
	VREncoding             string `json:"vr_encoding"`
	ByteOrder              string `json:"byte_order"`
	Encapsulated           bool   `json:"encapsulated"`
	Lossy                  bool   `json:"lossy"`
	LossyCompressionMethod string `json:"lossy_compression_method"`
	Deflated               bool   `json:"deflated"`
	Retired                bool   `json:"retired"`
}
type FileMetaSummary struct {
	MediaStorageSOPClassUID    string `json:"media_storage_sop_class_uid"`
	MediaStorageSOPInstanceUID string `json:"media_storage_sop_instance_uid"`
	ImplementationClassUID     string `json:"implementation_class_uid"`
	ImplementationVersionName  string `json:"implementation_version_name"`
	SourceApplicationAETitle   string `json:"source_application_ae_title"`
}
type EquipmentSummary struct {
	SpecificCharacterSet string `json:"specific_character_set"`
	Manufacturer         string `json:"manufacturer"`
	Model                string `json:"model"`
	Station              string `json:"station"`
	SoftwareVersions     string `json:"software_versions"`
}
type PatientSummary struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	BirthDate string `json:"birth_date"`
	Sex       string `json:"sex"`
}
type StudySummary struct {
	InstanceUID        string `json:"instance_uid"`
	ID                 string `json:"id"`
	Modality           string `json:"modality"`
	Date               string `json:"date"`
	Time               string `json:"time"`
	AccessionNumber    string `json:"accession_number"`
	Description        string `json:"description"`
	ReferringPhysician string `json:"referring_physician"`
	Institution        string `json:"institution"`
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
	ImageType            string `json:"image_type"`
	ContentDate          string `json:"content_date"`
	ContentTime          string `json:"content_time"`
	AcquisitionNumber    string `json:"acquisition_number"`
}
type PixelSummary struct {
	Rows                        int    `json:"rows"`
	Columns                     int    `json:"columns"`
	Frames                      int    `json:"frames"`
	Bytes                       int    `json:"bytes"`
	SamplesPerPixel             int    `json:"samples_per_pixel"`
	PhotometricInterpretation   string `json:"photometric_interpretation"`
	BitsAllocated               int    `json:"bits_allocated"`
	BitsStored                  int    `json:"bits_stored"`
	HighBit                     int    `json:"high_bit"`
	PixelRepresentation         int    `json:"pixel_representation"`
	PixelSpacing                string `json:"pixel_spacing"`
	WindowCenter                string `json:"window_center"`
	WindowWidth                 string `json:"window_width"`
	PlanarConfiguration         int    `json:"planar_configuration"`
	PixelAspectRatio            string `json:"pixel_aspect_ratio"`
	RescaleIntercept            string `json:"rescale_intercept"`
	RescaleSlope                string `json:"rescale_slope"`
	RescaleType                 string `json:"rescale_type"`
	VOILUTFunction              string `json:"voi_lut_function"`
	LossyImageCompression       string `json:"lossy_image_compression"`
	LossyImageCompressionRatio  string `json:"lossy_image_compression_ratio"`
	LossyImageCompressionMethod string `json:"lossy_image_compression_method"`
}
type ElementReport struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	Tag    string `json:"tag"`
	VR     string `json:"vr"`
	Value  string `json:"value"`
}

func Build(path string, parsed *parser.ParseResult, options Options) (Report, error) {
	if parsed == nil || parsed.Dataset == nil {
		return Report{}, fmt.Errorf("parsed dataset is required")
	}
	report := Report{
		File: FileSummary{Path: filepath.Clean(path)},
		FileMeta: FileMetaSummary{
			MediaStorageSOPClassUID:    fileMetaValue(parsed, tag.MediaStorageSOPClassUID),
			MediaStorageSOPInstanceUID: fileMetaValue(parsed, tag.MediaStorageSOPInstanceUID),
			ImplementationClassUID:     fileMetaValue(parsed, tag.ImplementationClassUID),
			ImplementationVersionName:  fileMetaValue(parsed, tag.ImplementationVersionName),
			SourceApplicationAETitle:   fileMetaValue(parsed, tag.SourceApplicationEntityTitle),
		},
		Equipment: EquipmentSummary{
			SpecificCharacterSet: stringValue(parsed.Dataset, tag.SpecificCharacterSet),
			Manufacturer:         stringValue(parsed.Dataset, tag.Manufacturer),
			Model:                stringValue(parsed.Dataset, tag.ManufacturerModelName),
			Station:              stringValue(parsed.Dataset, tag.StationName),
			SoftwareVersions:     stringValue(parsed.Dataset, tag.SoftwareVersions),
		},
		Patient: PatientSummary{
			Name:      stringValue(parsed.Dataset, tag.PatientName),
			ID:        stringValue(parsed.Dataset, tag.PatientID),
			BirthDate: stringValue(parsed.Dataset, tag.PatientBirthDate),
			Sex:       stringValue(parsed.Dataset, tag.PatientSex),
		},
		Study: StudySummary{
			InstanceUID:        stringValue(parsed.Dataset, tag.StudyInstanceUID),
			ID:                 stringValue(parsed.Dataset, tag.StudyID),
			Modality:           stringValue(parsed.Dataset, tag.Modality),
			Date:               stringValue(parsed.Dataset, tag.StudyDate),
			Time:               stringValue(parsed.Dataset, tag.StudyTime),
			AccessionNumber:    stringValue(parsed.Dataset, tag.AccessionNumber),
			Description:        stringValue(parsed.Dataset, tag.StudyDescription),
			ReferringPhysician: stringValue(parsed.Dataset, tag.ReferringPhysicianName),
			Institution:        stringValue(parsed.Dataset, tag.InstitutionName),
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
			ImageType:            stringValue(parsed.Dataset, tag.ImageType),
			ContentDate:          stringValue(parsed.Dataset, tag.ContentDate),
			ContentTime:          stringValue(parsed.Dataset, tag.ContentTime),
			AcquisitionNumber:    stringValue(parsed.Dataset, tag.AcquisitionNumber),
		},
		Pixel: PixelSummary{
			Rows:                        int(uint16Value(parsed.Dataset, tag.Rows)),
			Columns:                     int(uint16Value(parsed.Dataset, tag.Columns)),
			Frames:                      frameCount(parsed.Dataset),
			Bytes:                       elementLength(parsed.Dataset, tag.PixelData),
			SamplesPerPixel:             int(uint16Value(parsed.Dataset, tag.SamplesPerPixel)),
			PhotometricInterpretation:   stringValue(parsed.Dataset, tag.PhotometricInterpretation),
			BitsAllocated:               int(uint16Value(parsed.Dataset, tag.BitsAllocated)),
			BitsStored:                  int(uint16Value(parsed.Dataset, tag.BitsStored)),
			HighBit:                     int(uint16Value(parsed.Dataset, tag.HighBit)),
			PixelRepresentation:         int(uint16Value(parsed.Dataset, tag.PixelRepresentation)),
			PixelSpacing:                stringValue(parsed.Dataset, tag.PixelSpacing),
			WindowCenter:                stringValue(parsed.Dataset, tag.WindowCenter),
			WindowWidth:                 stringValue(parsed.Dataset, tag.WindowWidth),
			PlanarConfiguration:         int(uint16Value(parsed.Dataset, tag.PlanarConfiguration)),
			PixelAspectRatio:            stringValue(parsed.Dataset, tag.PixelAspectRatio),
			RescaleIntercept:            stringValue(parsed.Dataset, tag.RescaleIntercept),
			RescaleSlope:                stringValue(parsed.Dataset, tag.RescaleSlope),
			RescaleType:                 stringValue(parsed.Dataset, tag.RescaleType),
			VOILUTFunction:              stringValue(parsed.Dataset, tag.VOILUTFunction),
			LossyImageCompression:       stringValue(parsed.Dataset, tag.LossyImageCompression),
			LossyImageCompressionRatio:  stringValue(parsed.Dataset, tag.LossyImageCompressionRatio),
			LossyImageCompressionMethod: stringValue(parsed.Dataset, tag.LossyImageCompressionMethod),
		},
	}
	if parsed.TransferSyntax != nil {
		report.File.TransferSyntax = parsed.TransferSyntax.UID().UID()
		report.Encoding = EncodingSummary{
			UID:                    parsed.TransferSyntax.UID().UID(),
			Name:                   parsed.TransferSyntax.String(),
			VREncoding:             vrEncoding(parsed.TransferSyntax.IsExplicitVR()),
			ByteOrder:              byteOrder(parsed.TransferSyntax.Endian().IsLittle()),
			Encapsulated:           parsed.TransferSyntax.IsEncapsulated(),
			Lossy:                  parsed.TransferSyntax.IsLossy(),
			LossyCompressionMethod: parsed.TransferSyntax.LossyCompressionMethod(),
			Deflated:               parsed.TransferSyntax.IsDeflate(),
			Retired:                parsed.TransferSyntax.IsRetired(),
		}
	}
	if options.All {
		if parsed.FileMetaInformation != nil {
			for _, elem := range parsed.FileMetaInformation.Dataset().Elements() {
				report.Elements = append(report.Elements, describe("file_meta", elem.Tag().Format("g"), elem))
			}
		}
		for _, elem := range parsed.Dataset.Elements() {
			report.Elements = append(report.Elements, describe("dataset", elem.Tag().Format("g"), elem))
		}
	}
	for _, path := range options.Tags {
		elem, err := dicom.ResolveElement(parsed.Dataset, path)
		if err != nil {
			return Report{}, err
		}
		report.Tags = append(report.Tags, describe("dataset", path, elem))
	}
	return report, nil
}

func describe(source, path string, elem element.Element) ElementReport {
	value := elem.String()
	if stringElement, ok := elem.(*element.String); ok {
		value = stringElement.GetString()
	}
	return ElementReport{Source: source, Path: path, Tag: elem.Tag().Format("g"), VR: elem.ValueRepresentation().Code(), Value: value}
}

func stringValue(ds *dataset.Dataset, t *tag.Tag) string { return ds.TryGetString(t) }
func fileMetaValue(parsed *parser.ParseResult, t *tag.Tag) string {
	if parsed.FileMetaInformation == nil {
		return ""
	}
	return stringValue(parsed.FileMetaInformation.Dataset(), t)
}
func vrEncoding(explicit bool) string {
	if explicit {
		return "explicit"
	}
	return "implicit"
}
func byteOrder(littleEndian bool) string {
	if littleEndian {
		return "little-endian"
	}
	return "big-endian"
}
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
