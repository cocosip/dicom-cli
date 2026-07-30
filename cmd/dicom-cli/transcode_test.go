package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

func TestExecuteTranscodeFormatsListsRuntimeCodecsAndExperimentalHTJ2K(t *testing.T) {
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"transcode", "formats", "--json"}, runtime); code != 0 {
		t.Fatalf("transcode formats exit code = %d, want 0", code)
	}
	for _, want := range []string{"explicit-vr-little-endian", "1.2.840.10008.1.2.4.201", `"experimental":true`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("formats output does not contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestExecuteTranscodeWritesRequestedSyntax(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "output.dcm")
	runtime, _, _ := testRuntime()
	if code := Execute([]string{"transcode", "--to", "implicit-vr-little-endian", "--output", output, fixtures.SingleFrame}, runtime); code != 0 {
		t.Fatalf("transcode exit code = %d, want 0", code)
	}
	parsed, err := parser.ParseFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.TransferSyntax != transfer.ImplicitVRLittleEndian {
		t.Fatalf("transfer syntax = %s, want Implicit VR Little Endian", parsed.TransferSyntax.UID())
	}
}
