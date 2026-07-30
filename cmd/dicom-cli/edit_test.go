package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cocosip/dicom-cli/internal/dicom"
	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

func TestExecuteEditWritesNewFileAndPreservesInput(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "edited.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"edit", "--set", "PatientName=EDITED^PATIENT", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("edit exit code = %d, want 0", code)
	}
	after, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("input DICOM was modified")
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	elem, err := dicom.ResolveElement(parsed.Dataset, "PatientName")
	if err != nil || elem.(*element.String).GetString() != "EDITED^PATIENT" {
		t.Fatalf("edited PatientName = %#v, %v", elem, err)
	}
}

func TestExecuteEditUpdatesCharacterSet(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "utf8.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"edit", "--set", "PatientName=EDITED^PATIENT", "--charset", "ISO_IR 192", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("edit exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	charset, err := dicom.ResolveElement(parsed.Dataset, "SpecificCharacterSet")
	if err != nil || charset.(*element.String).GetString() != "ISO_IR 192" {
		t.Fatalf("SpecificCharacterSet = %#v, %v", charset, err)
	}
}

func TestExecuteEditAcceptsExplicitVRForPrivateTag(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "private.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"edit", "--set", "0011,0010=private", "--vr", "0011,0010=LO", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("edit exit code = %d, want 0", code)
	}
}

func TestExecuteEditRejectsRulesArgument(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"edit", "--rules", "unused.yaml", "--set", "PatientName=EDITED^PATIENT", "--output", filepath.Join(t.TempDir(), "edited.dcm"), fixtures.SingleFrame}, runtime); code != 2 {
		t.Fatalf("edit exit code = %d, want 2", code)
	}
}
