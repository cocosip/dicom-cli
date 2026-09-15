package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
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
	name, ok := parsed.Dataset.GetString(tag.PatientName)
	if !ok || name != "EDITED^PATIENT" {
		t.Fatalf("edited PatientName = %q, found=%t", name, ok)
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
	charset, ok := parsed.Dataset.GetString(tag.SpecificCharacterSet)
	if !ok || charset != "ISO_IR 192" {
		t.Fatalf("SpecificCharacterSet = %q, found=%t", charset, ok)
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
