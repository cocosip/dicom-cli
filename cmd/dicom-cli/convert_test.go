package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
)

func TestExecuteConvertImageAndJSONUseSharedDICOMExport(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pngPath := filepath.Join(t.TempDir(), "frame.png")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--format", "png", "--output", pngPath, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("convert image exit code = %d, want 0", code)
	}
	pngContent, err := os.ReadFile(pngPath)
	if err != nil || !bytes.HasPrefix(pngContent, []byte("\x89PNG")) {
		t.Fatalf("PNG output = %q, err=%v", pngContent, err)
	}

	jsonPath := filepath.Join(t.TempDir(), "metadata.json")
	runtime, _, _ = testRuntime()
	if code := Execute([]string{"convert", "--to", "json", "--output", jsonPath, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("convert --to json exit code = %d, want 0", code)
	}
	jsonContent, err := os.ReadFile(jsonPath)
	if err != nil || !strings.Contains(string(jsonContent), `"summary"`) {
		t.Fatalf("JSON output = %q, err=%v", jsonContent, err)
	}
}

func TestExecuteConvertImageRejectsMultipleFramesToStdout(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"convert", "image", "--all-frames", "--output", "-", fixtures.MultiFrame}, runtime); code != 2 {
		t.Fatalf("convert image --all-frames stdout exit code = %d, want 2", code)
	}
}
