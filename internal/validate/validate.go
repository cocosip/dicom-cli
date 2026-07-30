// Package validate collects DICOM conformance issues without short-circuiting.
package validate

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

type Severity string

const (
	Info    Severity = "info"
	Warning Severity = "warning"
	Error   Severity = "error"
)

type Issue struct {
	Source   string   `json:"source"`
	Path     string   `json:"path"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}
type Result struct {
	Issues []Issue `json:"issues"`
}

func Validate(parsed *parser.ParseResult, profiles ...rules.ValidateProfile) Result {
	result := Result{}
	if parsed == nil || parsed.Dataset == nil {
		return Result{Issues: []Issue{{Source: "dicom-cli-v1", Path: "file", Severity: Error, Message: "DICOM dataset could not be parsed"}}}
	}
	if parsed.FileMetaInformation == nil {
		result.add("dicom-cli-v1", "file_meta", Warning, "file meta information is missing")
	} else {
		meta := parsed.FileMetaInformation.Dataset()
		if parsed.TransferSyntax != nil {
			if value, ok := meta.GetString(tag.TransferSyntaxUID); !ok || value != parsed.TransferSyntax.UID().String() {
				result.add("dicom-cli-v1", "file_meta.TransferSyntaxUID", Error, "file meta transfer syntax does not match dataset encoding")
			}
		}
		for _, pair := range []struct {
			metaTag, datasetTag *tag.Tag
			path                string
		}{{tag.MediaStorageSOPClassUID, tag.SOPClassUID, "file_meta.MediaStorageSOPClassUID"}, {tag.MediaStorageSOPInstanceUID, tag.SOPInstanceUID, "file_meta.MediaStorageSOPInstanceUID"}} {
			metaValue, metaOK := meta.GetString(pair.metaTag)
			datasetValue, datasetOK := parsed.Dataset.GetString(pair.datasetTag)
			if metaOK && datasetOK && metaValue != datasetValue {
				result.add("dicom-cli-v1", pair.path, Error, "file meta value does not match dataset value")
			}
		}
	}
	if parsed.TransferSyntax == nil {
		result.add("dicom-cli-v1", "transfer_syntax", Error, "transfer syntax is missing")
	}
	for _, required := range []struct {
		name string
		tag  *tag.Tag
	}{
		{"PatientID", tag.PatientID}, {"StudyInstanceUID", tag.StudyInstanceUID}, {"SeriesInstanceUID", tag.SeriesInstanceUID},
		{"SOPClassUID", tag.SOPClassUID}, {"SOPInstanceUID", tag.SOPInstanceUID}, {"Modality", tag.Modality},
	} {
		elem, exists := parsed.Dataset.Get(required.tag)
		if !exists || elem.Count() == 0 {
			result.add("dicom-cli-v1", required.name, Error, "required value is missing")
		}
	}
	for _, elem := range parsed.Dataset.Elements() {
		if err := elem.Validate(); err != nil {
			result.add("dicom-cli-v1", elem.Tag().Format("g"), Error, err.Error())
		}
	}
	if parsed.Dataset.TryGetString(tag.SOPClassUID) == "1.2.840.10008.5.1.4.1.1.2" {
		for _, required := range []struct {
			name string
			tag  *tag.Tag
		}{{"Rows", tag.Rows}, {"Columns", tag.Columns}, {"BitsAllocated", tag.BitsAllocated}, {"BitsStored", tag.BitsStored}, {"HighBit", tag.HighBit}, {"PixelData", tag.PixelData}} {
			elem, exists := parsed.Dataset.Get(required.tag)
			if !exists || elem.Count() == 0 {
				result.add("dicom-cli-v1-ct-image-iod", required.name, Error, "required CT Image IOD value is missing")
			}
		}
	}
	for _, profile := range profiles {
		for _, rule := range profile.Rules {
			if evaluate(parsed, rule.When) && !evaluate(parsed, rule.Assert) {
				result.add("validate.profile", "validate.profile", Severity(rule.Severity), rule.Message)
			}
		}
	}
	return result
}

func (result *Result) add(source, path string, severity Severity, message string) {
	result.Issues = append(result.Issues, Issue{Source: source, Path: path, Severity: severity, Message: message})
}
func (result Result) Failure(strict bool) error {
	for _, issue := range result.Issues {
		if issue.Severity == Error || strict && issue.Severity == Warning {
			return apperr.Wrap(apperr.KindValidation, fmt.Errorf("validation failed"))
		}
	}
	return nil
}

func evaluate(parsed *parser.ParseResult, condition rules.Condition) bool {
	if len(condition.All) > 0 {
		for _, child := range condition.All {
			if !evaluate(parsed, child) {
				return false
			}
		}
		return true
	}
	if len(condition.Any) > 0 {
		for _, child := range condition.Any {
			if evaluate(parsed, child) {
				return true
			}
		}
		return false
	}
	elem, err := dicom.ResolveElement(parsed.Dataset, condition.Path)
	if condition.Exists != nil {
		return err == nil == *condition.Exists
	}
	if err != nil {
		return false
	}
	value := elementValue(elem)
	if condition.Equals != nil {
		return value == *condition.Equals
	}
	if condition.Matches != "" {
		matched, compileErr := regexp.MatchString(condition.Matches, value)
		return compileErr == nil && matched
	}
	if condition.Range != nil {
		number, parseErr := strconv.ParseFloat(value, 64)
		if parseErr != nil {
			return false
		}
		return (condition.Range.Min == nil || number >= *condition.Range.Min) && (condition.Range.Max == nil || number <= *condition.Range.Max)
	}
	return false
}

func elementValue(elem element.Element) string {
	if value, ok := elem.(*element.String); ok {
		return value.GetString()
	}
	return elem.String()
}
