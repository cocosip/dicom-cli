package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
)

func TestExecuteEditUsesSafeDefaultOutputDirectory(t *testing.T) {
	workingDirectory := t.TempDir()
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	runtime := Runtime{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: io.Discard, Getwd: func() (string, error) { return workingDirectory, nil }, UserConfigDir: func() (string, error) { return workingDirectory, nil }, LookupEnv: func(string) (string, bool) { return "", false }}
	if code := Execute([]string{"edit", "--set", "PatientName=EDITED^PATIENT", fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("edit exit code = %d, want 0", code)
	}
	want := filepath.Join(workingDirectory, "edit", filepath.Base(fixtures.SingleFrame))
	if strings.TrimSpace(stdout.String()) != want {
		t.Fatalf("output = %q, want %q", stdout.String(), want)
	}
}
