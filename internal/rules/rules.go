// Package rules defines and validates the versioned DICOM rules file.
package rules

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const VersionV1 = "v1"

const DefaultFileName = "dicom-cli-rules.yaml"

type File struct {
	Version        string                   `json:"version" yaml:"version"`
	Filters        map[string]Condition     `json:"filters,omitempty" yaml:"filters,omitempty"`
	Inspect        InspectSection           `json:"inspect,omitempty" yaml:"inspect,omitempty"`
	Anonymize      AnonymizeSection         `json:"anonymize,omitempty" yaml:"anonymize,omitempty"`
	ValidateRules  ValidateSection          `json:"validate,omitempty" yaml:"validate,omitempty"`
	DICOMTemplates map[string]DICOMTemplate `json:"dicom_templates,omitempty" yaml:"dicom_templates,omitempty"`
}

type InspectSection struct {
	Profiles map[string]InspectProfile `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}
type InspectProfile struct {
	Tags []string `json:"tags" yaml:"tags"`
}
type AnonymizeSection struct {
	Profiles map[string]AnonymizeProfile `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}
type AnonymizeProfile struct {
	Filter string          `json:"filter,omitempty" yaml:"filter,omitempty"`
	Rules  []AnonymizeRule `json:"rules" yaml:"rules"`
}
type AnonymizeRule struct {
	Path   string  `json:"path" yaml:"path"`
	Action string  `json:"action" yaml:"action"`
	Value  *string `json:"value,omitempty" yaml:"value,omitempty"`
}
type ValidateSection struct {
	Profiles map[string]ValidateProfile `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}
type ValidateProfile struct {
	Rules []ValidateRule `json:"rules" yaml:"rules"`
}
type ValidateRule struct {
	When     Condition `json:"when" yaml:"when"`
	Assert   Condition `json:"assert" yaml:"assert"`
	Severity string    `json:"severity" yaml:"severity"`
	Message  string    `json:"message" yaml:"message"`
}
type DICOMTemplate struct {
	Tags map[string]string `json:"tags" yaml:"tags"`
}

type Condition struct {
	Path    string      `json:"path,omitempty" yaml:"path,omitempty"`
	Exists  *bool       `json:"exists,omitempty" yaml:"exists,omitempty"`
	Equals  *string     `json:"equals,omitempty" yaml:"equals,omitempty"`
	Matches string      `json:"matches,omitempty" yaml:"matches,omitempty"`
	Range   *Range      `json:"range,omitempty" yaml:"range,omitempty"`
	All     []Condition `json:"all,omitempty" yaml:"all,omitempty"`
	Any     []Condition `json:"any,omitempty" yaml:"any,omitempty"`
}
type Range struct {
	Min *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	Max *float64 `json:"max,omitempty" yaml:"max,omitempty"`
}

func Load(path string) (File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var file File
	if filepath.Ext(path) == ".json" {
		decoder := json.NewDecoder(strings.NewReader(string(content)))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&file)
	} else {
		decoder := yaml.NewDecoder(strings.NewReader(string(content)))
		decoder.KnownFields(true)
		err = decoder.Decode(&file)
	}
	if err != nil {
		return File{}, fmt.Errorf("decode rules %q: %w", path, err)
	}
	if err := file.Validate(); err != nil {
		return File{}, err
	}
	return file, nil
}

// Example returns a valid starter rules document in the requested format.
func Example(format string) ([]byte, error) {
	file := File{Version: VersionV1, Filters: map[string]Condition{}, Inspect: InspectSection{Profiles: map[string]InspectProfile{}}, Anonymize: AnonymizeSection{Profiles: map[string]AnonymizeProfile{}}, ValidateRules: ValidateSection{Profiles: map[string]ValidateProfile{}}, DICOMTemplates: map[string]DICOMTemplate{}}
	switch format {
	case "yaml", "yml":
		return yaml.Marshal(file)
	case "json":
		content, err := json.MarshalIndent(file, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(content, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported rules format %q", format)
	}
}

func (file File) Validate() error {
	var problems []error
	if file.Version != VersionV1 {
		problems = append(problems, fmt.Errorf("version must be %q", VersionV1))
	}
	for name, condition := range file.Filters {
		problems = append(problems, validateCondition("filters."+name, condition)...)
	}
	for name, profile := range file.Inspect.Profiles {
		for index, path := range profile.Tags {
			problems = append(problems, validatePath(fmt.Sprintf("inspect.profiles.%s.tags[%d]", name, index), path))
		}
	}
	for name, profile := range file.Anonymize.Profiles {
		if profile.Filter != "" {
			if _, ok := file.Filters[profile.Filter]; !ok {
				problems = append(problems, fmt.Errorf("anonymize profile %q references missing filter %q", name, profile.Filter))
			}
		}
		seen := map[string]bool{}
		for index, rule := range profile.Rules {
			prefix := fmt.Sprintf("anonymize.profiles.%s.rules[%d]", name, index)
			problems = append(problems, validatePath(prefix+".path", rule.Path))
			if seen[rule.Path] {
				problems = append(problems, fmt.Errorf("%s duplicates an action for %q", prefix, rule.Path))
			}
			seen[rule.Path] = true
			switch rule.Action {
			case "delete", "clear", "remap_uid":
				if rule.Value != nil {
					problems = append(problems, fmt.Errorf("%s.action %q does not accept value", prefix, rule.Action))
				}
			case "replace":
				if rule.Value == nil {
					problems = append(problems, fmt.Errorf("%s.action replace requires value", prefix))
				}
			default:
				problems = append(problems, fmt.Errorf("%s.action %q is unknown", prefix, rule.Action))
			}
		}
	}
	for name, profile := range file.ValidateRules.Profiles {
		for index, rule := range profile.Rules {
			prefix := fmt.Sprintf("validate.profiles.%s.rules[%d]", name, index)
			problems = append(problems, validateCondition(prefix+".when", rule.When)...)
			problems = append(problems, validateCondition(prefix+".assert", rule.Assert)...)
			if rule.Severity != "info" && rule.Severity != "warning" && rule.Severity != "error" {
				problems = append(problems, fmt.Errorf("%s.severity is invalid", prefix))
			}
			if rule.Message == "" {
				problems = append(problems, fmt.Errorf("%s.message is required", prefix))
			}
		}
	}
	for name, template := range file.DICOMTemplates {
		for path := range template.Tags {
			problems = append(problems, validatePath("dicom_templates."+name+".tags", path))
		}
	}
	return errors.Join(problems...)
}

func validateCondition(prefix string, condition Condition) []error {
	operators := 0
	if condition.Exists != nil {
		operators++
	}
	if condition.Equals != nil {
		operators++
	}
	if condition.Matches != "" {
		operators++
	}
	if condition.Range != nil {
		operators++
	}
	if len(condition.All) > 0 {
		operators++
	}
	if len(condition.Any) > 0 {
		operators++
	}
	var problems []error
	if operators != 1 {
		return []error{fmt.Errorf("%s must declare exactly one condition operator", prefix)}
	}
	if len(condition.All) > 0 || len(condition.Any) > 0 {
		children := condition.All
		if len(children) == 0 {
			children = condition.Any
		}
		for index, child := range children {
			problems = append(problems, validateCondition(fmt.Sprintf("%s[%d]", prefix, index), child)...)
		}
		return problems
	}
	problems = append(problems, validatePath(prefix+".path", condition.Path))
	if condition.Matches != "" {
		if _, err := regexp.Compile(condition.Matches); err != nil {
			problems = append(problems, fmt.Errorf("%s.matches has invalid regular expression: %w", prefix, err))
		}
	}
	if condition.Range != nil {
		if condition.Range.Min == nil && condition.Range.Max == nil {
			problems = append(problems, fmt.Errorf("%s.range must include min or max", prefix))
		}
		if condition.Range.Min != nil && condition.Range.Max != nil && *condition.Range.Min > *condition.Range.Max {
			problems = append(problems, fmt.Errorf("%s.range min exceeds max", prefix))
		}
	}
	return problems
}

var tagSegment = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*|[0-9A-Fa-f]{4},?[0-9A-Fa-f]{4})(\[[0-9]+\])?$`)

func validatePath(prefix, path string) error {
	if path == "" {
		return fmt.Errorf("%s is required", prefix)
	}
	for _, segment := range strings.Split(path, ".") {
		if !tagSegment.MatchString(segment) {
			return fmt.Errorf("%s has invalid Tag path %q", prefix, path)
		}
	}
	return nil
}
