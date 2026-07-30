package validate

import (
	"testing"

	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

func TestValidateCollectsIndependentRequiredValueProblems(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Dataset.Remove(tag.PatientID)
	parsed.Dataset.Remove(tag.StudyInstanceUID)

	result := Validate(parsed)
	if !hasIssue(result.Issues, "PatientID", Error) || !hasIssue(result.Issues, "StudyInstanceUID", Error) {
		t.Fatalf("Issues = %#v, want PatientID and StudyInstanceUID errors", result.Issues)
	}
	if result.Failure(false) == nil {
		t.Fatal("Failure(false) = nil, want validation error")
	}
}

func TestValidateAddsCustomProfileAndStrictWarningFailure(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	ct := "CT"
	mr := "MR"
	profile := rules.ValidateProfile{Rules: []rules.ValidateRule{{
		When: rules.Condition{Path: "Modality", Equals: &ct}, Assert: rules.Condition{Path: "Modality", Equals: &mr},
		Severity: "warning", Message: "modality must be MR",
	}}}

	result := Validate(parsed, profile)
	if !hasIssue(result.Issues, "validate.profile", Warning) {
		t.Fatalf("Issues = %#v, want custom warning", result.Issues)
	}
	if result.Failure(false) != nil {
		t.Fatal("Failure(false) != nil, warnings must not fail default mode")
	}
	if result.Failure(true) == nil {
		t.Fatal("Failure(true) = nil, want strict warning failure")
	}
}

func TestValidateAppliesBuiltInCTImageIODRule(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Dataset.Remove(tag.Rows)

	result := Validate(parsed)
	if !hasIssue(result.Issues, "Rows", Error) {
		t.Fatalf("Issues = %#v, want built-in CT IOD Rows error", result.Issues)
	}
}

func hasIssue(issues []Issue, path string, severity Severity) bool {
	for _, issue := range issues {
		if issue.Path == path && issue.Severity == severity {
			return true
		}
	}
	return false
}
