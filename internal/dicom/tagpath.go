// Package dicom provides CLI-specific DICOM helpers over go-dicom types.
package dicom

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

type TagPath struct{ Segments []TagSegment }
type TagSegment struct {
	Token string
	Tag   *tag.Tag
	Index *int
}

// ResolveElement reads an element from a go-dicom dataset using CLI Tag syntax.
func ResolveElement(ds *dataset.Dataset, value string) (element.Element, error) {
	path, err := ParseTagPath(value)
	if err != nil {
		return nil, err
	}
	current := ds
	for index, segment := range path.Segments {
		element, ok := current.Get(segment.Tag)
		if !ok {
			return nil, fmt.Errorf("tag %q is not present", segment.Token)
		}
		if segment.Index == nil {
			if index != len(path.Segments)-1 {
				return nil, fmt.Errorf("tag %q requires a sequence index", segment.Token)
			}
			return element, nil
		}
		sequence, ok := element.(*dataset.Sequence)
		if !ok {
			return nil, fmt.Errorf("tag %q is not a sequence", segment.Token)
		}
		current = sequence.GetItem(*segment.Index)
		if current == nil {
			return nil, fmt.Errorf("sequence item %d is not present", *segment.Index)
		}
	}
	return nil, fmt.Errorf("tag path %q does not select an element", value)
}

func ParseTagPath(value string) (TagPath, error) {
	if value == "" {
		return TagPath{}, fmt.Errorf("tag path is empty")
	}
	parts := strings.Split(value, ".")
	path := TagPath{Segments: make([]TagSegment, 0, len(parts))}
	for _, part := range parts {
		segment, err := parseSegment(part)
		if err != nil {
			return TagPath{}, fmt.Errorf("invalid Tag path %q: %w", value, err)
		}
		path.Segments = append(path.Segments, segment)
	}
	return path, nil
}

func parseSegment(value string) (TagSegment, error) {
	if value == "" {
		return TagSegment{}, fmt.Errorf("empty segment")
	}
	segment := TagSegment{Token: value}
	if open := strings.IndexByte(value, '['); open >= 0 {
		if !strings.HasSuffix(value, "]") || strings.Count(value, "[") != 1 {
			return TagSegment{}, fmt.Errorf("invalid sequence index")
		}
		index, err := strconv.Atoi(value[open+1 : len(value)-1])
		if err != nil || index < 0 {
			return TagSegment{}, fmt.Errorf("invalid sequence index")
		}
		segment.Token, segment.Index = value[:open], &index
	}
	if segment.Token == "" {
		return TagSegment{}, fmt.Errorf("empty Tag token")
	}
	parsed, err := tag.Parse(segment.Token)
	if err == nil {
		segment.Tag = parsed
		return segment, nil
	}
	keyword, keywordErr := tag.ParseKeyword(segment.Token)
	if keywordErr != nil {
		return TagSegment{}, fmt.Errorf("invalid Tag %q: %w", segment.Token, keywordErr)
	}
	segment.Tag = &keyword
	return segment, nil
}
