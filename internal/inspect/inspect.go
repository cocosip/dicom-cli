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
	Pixel    PixelSummary    `json:"pixel"`
	Elements []ElementReport `json:"elements,omitempty"`
	Tags     []ElementReport `json:"tags,omitempty"`
}

type FileSummary struct {
	Path           string `json:"path"`
	TransferSyntax string `json:"transfer_syntax"`
}
type PatientSummary struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
type StudySummary struct {
	InstanceUID string `json:"instance_uid"`
	Modality    string `json:"modality"`
}
type SeriesSummary struct {
	InstanceUID string `json:"instance_uid"`
}
type PixelSummary struct {
	Rows    int `json:"rows"`
	Columns int `json:"columns"`
	Frames  int `json:"frames"`
	Bytes   int `json:"bytes"`
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
		File:    FileSummary{Path: filepath.Clean(path)},
		Patient: PatientSummary{Name: stringValue(parsed.Dataset, tag.PatientName), ID: stringValue(parsed.Dataset, tag.PatientID)},
		Study:   StudySummary{InstanceUID: stringValue(parsed.Dataset, tag.StudyInstanceUID), Modality: stringValue(parsed.Dataset, tag.Modality)},
		Series:  SeriesSummary{InstanceUID: stringValue(parsed.Dataset, tag.SeriesInstanceUID)},
		Pixel:   PixelSummary{Rows: int(uint16Value(parsed.Dataset, tag.Rows)), Columns: int(uint16Value(parsed.Dataset, tag.Columns)), Frames: frameCount(parsed.Dataset), Bytes: elementLength(parsed.Dataset, tag.PixelData)},
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
