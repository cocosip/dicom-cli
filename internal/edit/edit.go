// Package edit applies explicit, non-destructive DICOM dataset mutations.
package edit

import (
	"fmt"
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
			valueRepresentation, err := resolveValueRepresentation(parent, target, operation.VR)
			if err != nil {
				return err
			}
			values := strings.Split(operation.Value, "\\")
			if valueRepresentation.IsString() {
				if err := parent.AddOrUpdateValueWithVR(target, valueRepresentation, values); err != nil {
					return err
				}
			} else {
				var replacement element.Element
				if existing, ok := parent.Get(target); ok {
					replacement, err = element.ReplaceCanonicalStrings(existing, target, valueRepresentation, values)
				} else {
					context := element.CanonicalValueContext{}
					if syntax := parent.InternalTransferSyntax(); syntax != nil {
						context.Endian = syntax.Endian()
					}
					replacement, err = element.ReplaceCanonicalStringsWithContext(target, valueRepresentation, values, context)
				}
				if err != nil {
					return err
				}
				if err := parent.AddOrUpdate(replacement); err != nil {
					return err
				}
			}
		case Clear:
			elem, exists := parent.Get(target)
			if !exists {
				return fmt.Errorf("tag %q is not present", operation.Path)
			}
			if elem.ValueRepresentation() == vr.SQ {
				if err := parent.AddOrUpdate(dataset.NewSequence(target)); err != nil {
					return err
				}
			} else if err := parent.AddOrUpdateValueWithVR(target, elem.ValueRepresentation(), nil); err != nil {
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
			old, ok := parent.GetString(target)
			if !ok || elem.ValueRepresentation() != vr.UI {
				return fmt.Errorf("tag %q is not a UID", operation.Path)
			}
			generated := generator.next()
			mapping[old] = generated
			if err := parent.AddOrUpdateValue(target, generated); err != nil {
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
	return ds.SetSpecificCharacterSet(outputCharset)
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
		if !elem.ValueRepresentation().IsStringEncoded() {
			continue
		}
		var values []string
		if len(inputCharsets) > 0 {
			decoded, err := charset.DecodeString(elem.Buffer().Data(), charset.GetEncodings(inputCharsets))
			if err != nil {
				return fmt.Errorf("decode %s with %s: %w", elem.Tag(), inputCharsets[0], err)
			}
			values = strings.Split(strings.TrimRight(decoded, "\x00 "), "\\")
		} else {
			var err error
			values, err = element.CanonicalStrings(elem)
			if err != nil {
				return fmt.Errorf("read %s: %w", elem.Tag(), err)
			}
		}
		replacement, err := element.ReplaceCanonicalStringsWithContext(
			elem.Tag(),
			elem.ValueRepresentation(),
			values,
			element.CanonicalValueContext{TextEncodings: []encoding.Encoding{outputEncoding}},
		)
		if err != nil {
			return fmt.Errorf("encode %s: %w", elem.Tag(), err)
		}
		if err := ds.AddOrUpdate(replacement); err != nil {
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

func resolveValueRepresentation(parent *dataset.Dataset, target *tag.Tag, explicitVR string) (*vr.VR, error) {
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
	return parsedVR, nil
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
		if elem.ValueRepresentation() != vr.UI {
			continue
		}
		values, ok := ds.GetStrings(elem.Tag())
		if !ok {
			continue
		}
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
			_ = ds.AddOrUpdateValue(elem.Tag(), values)
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
