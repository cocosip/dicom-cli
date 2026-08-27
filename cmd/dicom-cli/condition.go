package main

import (
	"regexp"
	"strconv"

	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
)

func matchCondition(ds *dataset.Dataset, condition rules.Condition) bool {
	if len(condition.All) > 0 {
		for _, child := range condition.All {
			if !matchCondition(ds, child) {
				return false
			}
		}
		return true
	}
	if len(condition.Any) > 0 {
		for _, child := range condition.Any {
			if matchCondition(ds, child) {
				return true
			}
		}
		return false
	}
	elem, err := dicom.ResolveElement(ds, condition.Path)
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
		return parseErr == nil && (condition.Range.Min == nil || number >= *condition.Range.Min) && (condition.Range.Max == nil || number <= *condition.Range.Max)
	}
	return false
}

func elementValue(elem element.Element) string {
	if value, ok := elem.(*element.String); ok {
		return value.GetString()
	}
	return elem.String()
}
