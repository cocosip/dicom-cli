// Package anonymize adapts go-dicom's PS3.15 anonymizer for CLI rule profiles.
package anonymize

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/edit"
	"github.com/cocosip/dicom-cli/internal/rules"
	goanonymizer "github.com/cocosip/go-dicom/pkg/dicom/anonymizer"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

type Options struct {
	ProfileOptions []string
	Rules          []rules.AnonymizeRule
}

type Change struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type Result struct {
	Dataset     *dataset.Dataset
	Changes     []Change
	UIDMappings map[string]string
}

// Engine owns the per-command UID map. It must be created once for a batch.
type Engine struct {
	profile    *goanonymizer.SecurityProfile
	rules      []rules.AnonymizeRule
	uids       map[string]string
	retainUIDs bool
}

func NewEngine(options Options) (*Engine, error) {
	profileOptions, err := parseProfileOptions(options.ProfileOptions)
	if err != nil {
		return nil, err
	}
	profile := goanonymizer.NewSecurityProfile(goanonymizer.BasicProfile | profileOptions)
	return &Engine{profile: profile, rules: options.Rules, uids: map[string]string{}, retainUIDs: profileOptions&goanonymizer.RetainUIDs != 0}, nil
}

func (engine *Engine) Anonymize(source *dataset.Dataset) (Result, error) {
	before := snapshot(source)
	copy := source.Clone()
	base := goanonymizer.NewAnonymizer(engine.profile)
	base.ReplacedUIDs = engine.uids
	if err := base.AnonymizeInPlace(copy); err != nil {
		return Result{}, err
	}
	if err := restorePrivateElements(source, copy); err != nil {
		return Result{}, err
	}
	if engine.retainUIDs {
		if err := restoreUIDElements(source, copy); err != nil {
			return Result{}, err
		}
	}
	for _, rule := range engine.rules {
		if err := applyRule(copy, rule, engine.uids); err != nil {
			return Result{}, err
		}
	}
	return Result{Dataset: copy, Changes: changes(before, snapshot(copy)), UIDMappings: cloneMappings(engine.uids)}, nil
}

func restoreUIDElements(source, target *dataset.Dataset) error {
	for _, sourceElement := range source.Elements() {
		if sourceElement.ValueRepresentation().Code() == vr.CodeUI {
			cloned, err := cloneElement(sourceElement)
			if err != nil {
				return err
			}
			if err := target.AddOrUpdate(cloned); err != nil {
				return err
			}
			continue
		}
		sourceSequence, isSequence := sourceElement.(*dataset.Sequence)
		if !isSequence {
			continue
		}
		targetElement, exists := target.Get(sourceElement.Tag())
		if !exists {
			continue
		}
		targetSequence, ok := targetElement.(*dataset.Sequence)
		if !ok {
			continue
		}
		count := sourceSequence.Count()
		if targetSequence.Count() < count {
			count = targetSequence.Count()
		}
		for index := 0; index < count; index++ {
			if err := restoreUIDElements(sourceSequence.GetItem(index), targetSequence.GetItem(index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func restorePrivateElements(source, target *dataset.Dataset) error {
	for _, sourceElement := range source.Elements() {
		if sourceElement.Tag().Group()%2 == 1 {
			cloned, err := cloneElement(sourceElement)
			if err != nil {
				return err
			}
			if err := target.AddOrUpdate(cloned); err != nil {
				return err
			}
			continue
		}
		sourceSequence, isSequence := sourceElement.(*dataset.Sequence)
		if !isSequence {
			continue
		}
		targetElement, exists := target.Get(sourceElement.Tag())
		if !exists {
			continue
		}
		targetSequence, ok := targetElement.(*dataset.Sequence)
		if !ok {
			continue
		}
		count := sourceSequence.Count()
		if targetSequence.Count() < count {
			count = targetSequence.Count()
		}
		for index := 0; index < count; index++ {
			if err := restorePrivateElements(sourceSequence.GetItem(index), targetSequence.GetItem(index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func cloneElement(value element.Element) (element.Element, error) {
	wrapper := dataset.New()
	if err := wrapper.Add(value); err != nil {
		return nil, err
	}
	cloned, ok := wrapper.Clone().Get(value.Tag())
	if !ok {
		return nil, fmt.Errorf("clone did not retain %s", value.Tag())
	}
	return cloned, nil
}

func (engine *Engine) UIDMappings() map[string]string { return cloneMappings(engine.uids) }

var supportedOptions = map[string]goanonymizer.SecurityProfileOptions{
	"retain-safe-private":            goanonymizer.RetainSafePrivate,
	"retain-uids":                    goanonymizer.RetainUIDs,
	"retain-device-identity":         goanonymizer.RetainDeviceIdent,
	"retain-institution-identity":    goanonymizer.RetainInstitutionIdent,
	"retain-patient-characteristics": goanonymizer.RetainPatientChars,
	"retain-longitudinal-temporal-information-with-full-dates":     goanonymizer.RetainLongFullDates,
	"retain-longitudinal-temporal-information-with-modified-dates": goanonymizer.RetainLongModifDates,
	"clean-descriptors":        goanonymizer.CleanDesc,
	"clean-structured-content": goanonymizer.CleanStructdCont,
	"clean-graphics":           goanonymizer.CleanGraph,
}

func parseProfileOptions(names []string) (goanonymizer.SecurityProfileOptions, error) {
	var selected goanonymizer.SecurityProfileOptions
	for _, name := range names {
		option, ok := supportedOptions[name]
		if !ok {
			return 0, fmt.Errorf("unsupported anonymize profile option %q", name)
		}
		selected |= option
	}
	return selected, nil
}

func applyRule(ds *dataset.Dataset, rule rules.AnonymizeRule, uidMappings map[string]string) error {
	if _, err := resolve(ds, rule.Path); err != nil {
		if strings.Contains(err.Error(), "is not present") {
			return nil
		}
		return err
	}
	switch rule.Action {
	case "delete":
		return edit.Apply(ds, []edit.Operation{{Kind: edit.Delete, Path: rule.Path}}, edit.Options{})
	case "clear":
		return edit.Apply(ds, []edit.Operation{{Kind: edit.Clear, Path: rule.Path}}, edit.Options{})
	case "replace":
		if rule.Value == nil {
			return fmt.Errorf("replace rule for %q has no value", rule.Path)
		}
		return edit.Apply(ds, []edit.Operation{{Kind: edit.Set, Path: rule.Path, Value: *rule.Value}}, edit.Options{})
	case "remap_uid":
		return remapUID(ds, rule.Path, uidMappings)
	default:
		return fmt.Errorf("unsupported anonymize action %q", rule.Action)
	}
}

func remapUID(ds *dataset.Dataset, path string, uidMappings map[string]string) error {
	parent, target, err := parentFor(ds, path)
	if err != nil {
		return err
	}
	elem, ok := parent.Get(target)
	if !ok {
		return nil
	}
	temporary := dataset.New()
	if err := temporary.Add(elem); err != nil {
		return err
	}
	profile := goanonymizer.NewSecurityProfile(0)
	if err := profile.AddRule("^"+target.String()[1:len(target.String())-1]+"$", goanonymizer.ActionU); err != nil {
		return err
	}
	anonymizer := goanonymizer.NewAnonymizer(profile)
	anonymizer.ReplacedUIDs = uidMappings
	if err := anonymizer.AnonymizeInPlace(temporary); err != nil {
		return err
	}
	replaced, ok := temporary.Get(target)
	if !ok {
		return fmt.Errorf("UID %q was removed", path)
	}
	return parent.AddOrUpdate(replaced)
}

func resolve(ds *dataset.Dataset, path string) (element.Element, error) {
	parent, target, err := parentFor(ds, path)
	if err != nil {
		return nil, err
	}
	elem, ok := parent.Get(target)
	if !ok {
		return nil, fmt.Errorf("tag %q is not present", path)
	}
	return elem, nil
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

func snapshot(ds *dataset.Dataset) map[string]string {
	values := map[string]string{}
	collect(ds, "", values)
	return values
}

func collect(ds *dataset.Dataset, prefix string, values map[string]string) {
	for _, elem := range ds.Elements() {
		path := elem.Tag().String()
		if prefix != "" {
			path = prefix + "." + path
		}
		if sequence, ok := elem.(*dataset.Sequence); ok {
			values[path] = "sequence"
			for index := 0; index < sequence.Count(); index++ {
				collect(sequence.GetItem(index), fmt.Sprintf("%s[%d]", path, index), values)
			}
			continue
		}
		if stringElement, ok := elem.(*element.String); ok {
			values[path] = strings.Join(stringElement.GetValues(), "\\")
			continue
		}
		values[path] = fmt.Sprint(elem)
	}
}

func changes(before, after map[string]string) []Change {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	paths := make([]string, 0, len(keys))
	for path := range keys {
		if before[path] != after[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	result := make([]Change, 0, len(paths))
	for _, path := range paths {
		result = append(result, Change{Path: path, Before: before[path], After: after[path]})
	}
	return result
}

func cloneMappings(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for original, replacement := range values {
		result[original] = replacement
	}
	return result
}
