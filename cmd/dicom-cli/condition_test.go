package main

import (
	"testing"

	"github.com/cocosip/dicom-cli/internal/rules"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
)

func TestMatchConditionReadsTypedPersonName(t *testing.T) {
	ds := dataset.New()
	if err := ds.AddValue(tag.PatientName, "DOE^JANE"); err != nil {
		t.Fatal(err)
	}
	want := "DOE^JANE"
	if !matchCondition(ds, rules.Condition{Path: "PatientName", Equals: &want}) {
		t.Fatal("matchCondition() = false for a typed PN value")
	}
}
