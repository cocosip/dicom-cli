// Package edit applies explicit, non-destructive DICOM dataset mutations.
package edit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/go-dicom/pkg/dicom/charset"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/dict"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/uid"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"golang.org/x/text/encoding"
)

type Kind string

const (
	Set         Kind = "set"
	Clear       Kind = "clear"
	Delete      Kind = "delete"
	GenerateUID Kind = "generate_uid"
)

type Operation struct {
	Kind  Kind
	Path  string
	Value string
	VR    string
}
type Options struct {
	UIDRoot   string
	RemapUIDs bool
}

func Apply(ds *dataset.Dataset, operations []Operation, options Options) error {
	mapping := map[string]string{}
	generator := uidGenerator{root: options.UIDRoot}
	for _, operation := range operations {
		parent, target, err := parentFor(ds, operation.Path)
		if err != nil {
			return err
		}
		switch operation.Kind {
		case Set:
			elem, err := newStringElement(parent, target, operation.Value, operation.VR)
			if err != nil {
				return err
			}
			if err := parent.AddOrUpdate(elem); err != nil {
				return err
			}
		case Clear:
			elem, exists := parent.Get(target)
			if !exists {
				return fmt.Errorf("tag %q is not present", operation.Path)
			}
			empty, err := emptyElement(target, elem.ValueRepresentation())
			if err != nil {
				return err
			}
			if err := parent.AddOrUpdate(empty); err != nil {
				return err
			}
		case Delete:
			if !parent.Remove(target) {
				return fmt.Errorf("tag %q is not present", operation.Path)
			}
		case GenerateUID:
			elem, exists := parent.Get(target)
			if !exists {
				return fmt.Errorf("tag %q is not present", operation.Path)
			}
			old, ok := elem.(*element.String)
			if !ok || elem.ValueRepresentation() != vr.UI {
				return fmt.Errorf("tag %q is not a UID", operation.Path)
			}
			generated := generator.next()
			mapping[old.GetString()] = generated
			if err := parent.AddOrUpdate(element.NewString(target, vr.UI, []string{generated})); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown edit operation %q", operation.Kind)
		}
	}
	if options.RemapUIDs {
		remapUIDs(ds, mapping, &generator)
	}
	return nil
}

// ConvertCharacterSet rewrites every text element and updates SpecificCharacterSet.
// inputCharset overrides the encoding declared by the input file when non-empty.
func ConvertCharacterSet(ds *dataset.Dataset, outputCharset, inputCharset string) error {
	output, ok := charset.GetCharsetInfo(outputCharset)
	if !ok {
		return fmt.Errorf("unsupported output character set %q", outputCharset)
	}
	var inputEncodings []string
	if inputCharset != "" {
		if _, ok := charset.GetCharsetInfo(inputCharset); !ok {
			return fmt.Errorf("unsupported input character set %q", inputCharset)
		}
		inputEncodings = []string{inputCharset}
	}
	if err := convertDatasetCharacterSet(ds, output.Encoding, inputEncodings); err != nil {
		return err
	}
	return ds.AddOrUpdate(element.NewString(tag.SpecificCharacterSet, vr.CS, []string{outputCharset}))
}

func convertDatasetCharacterSet(ds *dataset.Dataset, outputEncoding encoding.Encoding, inputCharsets []string) error {
	for _, elem := range ds.Elements() {
		if sequence, ok := elem.(*dataset.Sequence); ok {
			for _, item := range sequence.GetItems() {
				if err := convertDatasetCharacterSet(item, outputEncoding, inputCharsets); err != nil {
					return err
				}
			}
			continue
		}
		stringElement, ok := elem.(*element.String)
		if !ok || !elem.ValueRepresentation().IsStringEncoded() {
			continue
		}
		values := stringElement.GetValues()
		if len(inputCharsets) > 0 {
			decoded, err := charset.DecodeString(elem.Buffer().Data(), charset.GetEncodings(inputCharsets))
			if err != nil {
				return fmt.Errorf("decode %s with %s: %w", elem.Tag(), inputCharsets[0], err)
			}
			values = strings.Split(strings.TrimRight(decoded, "\x00 "), "\\")
		}
		if _, err := charset.EncodeString(strings.Join(values, "\\"), []encoding.Encoding{outputEncoding}); err != nil {
			return fmt.Errorf("encode %s: %w", elem.Tag(), err)
		}
		if err := ds.AddOrUpdate(element.NewStringWithEncoding(elem.Tag(), elem.ValueRepresentation(), values, outputEncoding)); err != nil {
			return err
		}
	}
	return nil
}

func parentFor(ds *dataset.Dataset, path string) (*dataset.Dataset, *tag.Tag, error) {
	parsed, err := dicom.ParseTagPath(path)
	if err != nil {
		return nil, nil, err
	}
	if len(parsed.Segments) == 0 {
		return nil, nil, fmt.Errorf("tag path is empty")
	}
	current := ds
	for _, segment := range parsed.Segments[:len(parsed.Segments)-1] {
		if segment.Index == nil {
			return nil, nil, fmt.Errorf("tag %q requires a sequence index", segment.Token)
		}
		elem, ok := current.Get(segment.Tag)
		if !ok {
			return nil, nil, fmt.Errorf("tag %q is not present", segment.Token)
		}
		sequence, ok := elem.(*dataset.Sequence)
		if !ok {
			return nil, nil, fmt.Errorf("tag %q is not a sequence", segment.Token)
		}
		current = sequence.GetItem(*segment.Index)
		if current == nil {
			return nil, nil, fmt.Errorf("sequence item %d is not present", *segment.Index)
		}
	}
	target := parsed.Segments[len(parsed.Segments)-1]
	if target.Index != nil {
		return nil, nil, fmt.Errorf("final tag %q cannot select a sequence item", target.Token)
	}
	return current, target.Tag, nil
}

func newStringElement(parent *dataset.Dataset, target *tag.Tag, value, explicitVR string) (element.Element, error) {
	selected := explicitVR
	if selected == "" {
		if existing, ok := parent.Get(target); ok {
			selected = existing.ValueRepresentation().Code()
		} else if entry := dict.Default().Lookup(target); entry != nil && len(entry.ValueRepresentations()) == 1 {
			selected = entry.ValueRepresentations()[0].Code()
		} else {
			return nil, fmt.Errorf("tag %s requires an explicit VR", target)
		}
	}
	parsedVR, err := vr.Parse(strings.ToUpper(selected))
	if err != nil {
		return nil, err
	}
	values := strings.Split(value, "\\")
	if parsedVR.IsString() {
		if err := parsedVR.ValidateString(value); err != nil {
			return nil, err
		}
		return element.NewString(target, parsedVR, values), nil
	}
	return newNumericElement(target, parsedVR, values)
}

func newNumericElement(target *tag.Tag, valueRepresentation *vr.VR, values []string) (element.Element, error) {
	switch valueRepresentation.Code() {
	case vr.CodeUS:
		parsed, err := parseUnsigned(values, 16)
		if err != nil {
			return nil, err
		}
		cast := make([]uint16, len(parsed))
		for i := range parsed {
			cast[i] = uint16(parsed[i])
		}
		return element.NewUnsignedShort(target, cast), nil
	case vr.CodeSS:
		parsed, err := parseSigned(values, 16)
		if err != nil {
			return nil, err
		}
		cast := make([]int16, len(parsed))
		for i := range parsed {
			cast[i] = int16(parsed[i])
		}
		return element.NewSignedShort(target, cast), nil
	case vr.CodeUL:
		parsed, err := parseUnsigned(values, 32)
		if err != nil {
			return nil, err
		}
		cast := make([]uint32, len(parsed))
		for i := range parsed {
			cast[i] = uint32(parsed[i])
		}
		return element.NewUnsignedLong(target, cast), nil
	case vr.CodeSL:
		parsed, err := parseSigned(values, 32)
		if err != nil {
			return nil, err
		}
		cast := make([]int32, len(parsed))
		for i := range parsed {
			cast[i] = int32(parsed[i])
		}
		return element.NewSignedLong(target, cast), nil
	case vr.CodeSV:
		parsed, err := parseSigned(values, 64)
		if err != nil {
			return nil, err
		}
		return element.NewSignedVeryLong(target, parsed), nil
	case vr.CodeUV:
		parsed, err := parseUnsigned(values, 64)
		if err != nil {
			return nil, err
		}
		return element.NewUnsignedVeryLong(target, parsed), nil
	case vr.CodeFL:
		parsed, err := parseFloat32(values)
		if err != nil {
			return nil, err
		}
		return element.NewFloat(target, parsed), nil
	case vr.CodeFD:
		parsed, err := parseFloat64(values)
		if err != nil {
			return nil, err
		}
		return element.NewDouble(target, parsed), nil
	default:
		return nil, fmt.Errorf("VR %s is not supported by --set", valueRepresentation.Code())
	}
}

func emptyElement(target *tag.Tag, valueRepresentation *vr.VR) (element.Element, error) {
	if valueRepresentation.IsString() {
		return element.NewString(target, valueRepresentation, nil), nil
	}
	if valueRepresentation == vr.SQ {
		return dataset.NewSequence(target), nil
	}
	return newNumericElement(target, valueRepresentation, nil)
}

func parseUnsigned(values []string, bits int) ([]uint64, error) {
	parsed := make([]uint64, len(values))
	for i, value := range values {
		number, err := strconv.ParseUint(value, 10, bits)
		if err != nil {
			return nil, fmt.Errorf("value %q is not an unsigned %d-bit integer: %w", value, bits, err)
		}
		parsed[i] = number
	}
	return parsed, nil
}
func parseSigned(values []string, bits int) ([]int64, error) {
	parsed := make([]int64, len(values))
	for i, value := range values {
		number, err := strconv.ParseInt(value, 10, bits)
		if err != nil {
			return nil, fmt.Errorf("value %q is not a signed %d-bit integer: %w", value, bits, err)
		}
		parsed[i] = number
	}
	return parsed, nil
}
func parseFloat32(values []string) ([]float32, error) {
	parsed := make([]float32, len(values))
	for i, value := range values {
		number, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, fmt.Errorf("value %q is not a 32-bit float: %w", value, err)
		}
		parsed[i] = float32(number)
	}
	return parsed, nil
}
func parseFloat64(values []string) ([]float64, error) {
	parsed := make([]float64, len(values))
	for i, value := range values {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("value %q is not a 64-bit float: %w", value, err)
		}
		parsed[i] = number
	}
	return parsed, nil
}

type uidGenerator struct {
	root     string
	sequence int64
}

func (generator *uidGenerator) next() string {
	if generator.root == "" {
		return uid.GenerateDerivedFromUUID().UID()
	}
	generator.sequence++
	return uid.GenerateFromRoot(generator.root, generator.sequence).UID()
}
func remapUIDs(ds *dataset.Dataset, mapping map[string]string, generator *uidGenerator) {
	for _, elem := range ds.Elements() {
		if sequence, ok := elem.(*dataset.Sequence); ok {
			for _, item := range sequence.GetItems() {
				remapUIDs(item, mapping, generator)
			}
			continue
		}
		stringElement, ok := elem.(*element.String)
		if !ok || elem.ValueRepresentation() != vr.UI {
			continue
		}
		values := stringElement.GetValues()
		changed := false
		for index, value := range values {
			mapped, ok := mapping[value]
			if !ok && !mappedValue(mapping, value) {
				mapped = generator.next()
				mapping[value] = mapped
			}
			if mapped != "" && value != mapped {
				values[index], changed = mapped, true
			}
		}
		if changed {
			_ = ds.AddOrUpdate(element.NewString(elem.Tag(), vr.UI, values))
		}
	}
}

func mappedValue(mapping map[string]string, value string) bool {
	for _, mapped := range mapping {
		if mapped == value {
			return true
		}
	}
	return false
}
