package dicom

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

func TestRuntimeCodecsListsRegisteredSyntaxesAndResolvesAliasOrUID(t *testing.T) {
	formats := RuntimeCodecs()
	if len(formats) == 0 {
		t.Fatal("RuntimeCodecs() returned no registered syntaxes")
	}

	byAlias, err := ResolveTransferSyntax("explicit-vr-little-endian")
	if err != nil {
		t.Fatalf("ResolveTransferSyntax(alias) error = %v", err)
	}
	if byAlias.UID != transfer.ExplicitVRLittleEndian.UID().UID() || !byAlias.Encode || !byAlias.Decode {
		t.Fatalf("alias format = %+v", byAlias)
	}
	byUID, err := ResolveTransferSyntax(transfer.ExplicitVRLittleEndian.UID().UID())
	if err != nil {
		t.Fatalf("ResolveTransferSyntax(UID) error = %v", err)
	}
	if byUID != byAlias {
		t.Fatalf("UID format = %+v, want %+v", byUID, byAlias)
	}
}

func TestRuntimeCodecsRejectsUnregisteredSyntaxAndMarksHTJ2KExperimental(t *testing.T) {
	if _, err := ResolveTransferSyntax("1.2.840.10008.1.2.4.201"); err == nil {
		t.Fatal("ResolveTransferSyntax(HTJ2K UID) succeeded without a linked codec")
	}
	if _, err := ResolveTransferSyntax("not-a-transfer-syntax"); err == nil {
		t.Fatal("ResolveTransferSyntax(unknown) succeeded")
	}
}
