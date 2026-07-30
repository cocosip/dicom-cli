package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAcceptsAllConditionOperators(t *testing.T) {
	file := File{
		Version: VersionV1,
		Filters: map[string]Condition{
			"ct": {All: []Condition{
				{Path: "Modality", Equals: stringPtr("CT")},
				{Any: []Condition{
					{Path: "SliceThickness", Exists: boolPtr(true)},
					{Path: "0040,A730[0].0040,A160", Matches: "^[A-Z]+$"},
					{Path: "InstanceNumber", Range: &Range{Min: floatPtr(1), Max: floatPtr(10)}},
				}},
			}},
		},
	}
	if err := file.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateAggregatesStaticRuleErrors(t *testing.T) {
	file := File{
		Version: "v2",
		Filters: map[string]Condition{
			"bad": {Path: "not a path", Matches: "["},
		},
		Anonymize: AnonymizeSection{Profiles: map[string]AnonymizeProfile{
			"default": {Filter: "missing", Rules: []AnonymizeRule{
				{Path: "PatientName", Action: "replace"},
				{Path: "PatientName", Action: "clear"},
				{Path: "PatientID", Action: "unknown"},
			}},
		}},
	}
	err := file.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want aggregated validation errors")
	}
	for _, want := range []string{"version", "not a path", "regular expression", "missing", "value", "duplicate", "unknown"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want %q", err, want)
		}
	}
}

func TestValidateAcceptsAnonymizeProfileOptions(t *testing.T) {
	file := File{
		Version: VersionV1,
		Anonymize: AnonymizeSection{Profiles: map[string]AnonymizeProfile{
			"research": {Options: []string{"retain-uids", "retain-longitudinal-temporal-information-with-full-dates"}},
		}},
	}
	if err := file.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadRejectsUnknownFieldsInYAMLAndJSON(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct{ name, content string }{
		{name: "yaml", content: "version: v1\nunknown: true\n"},
		{name: "json", content: "{\"version\":\"v1\",\"unknown\":true}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, "rules."+test.name)
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load() error = nil, want unknown-field error")
			}
		})
	}
}

func boolPtr(value bool) *bool        { return &value }
func stringPtr(value string) *string  { return &value }
func floatPtr(value float64) *float64 { return &value }
