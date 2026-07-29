package dicom

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cocosip/dicom-cli/internal/testutil"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

func TestDirectGoDicomReadWriteDoesNotModifySource(t *testing.T) {
	fixtures, err := testutil.CreateDICOMFixtures(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "copy.dcm")
	if err := writer.WriteFile(copyPath, parsed.Dataset, writer.WithTransferSyntax(parsed.TransferSyntax)); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(copyPath); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(fixtures.SingleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
}
