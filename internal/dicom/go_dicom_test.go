package dicom

import (
	"strings"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/dict"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/uid"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

func TestGoDicomDictionaryProvidesVRForStandardTags(t *testing.T) {
	entry := dict.Default().Lookup(tag.PatientName)
	if entry == nil {
		t.Fatal("PatientName is missing from the DICOM dictionary")
	}
	vrs := entry.ValueRepresentations()
	if len(vrs) != 1 || vrs[0].Code() != "PN" {
		t.Fatalf("PatientName VRs = %#v, want [PN]", entry.VRs())
	}
	if entry := dict.Default().Lookup(tag.New(0x0011, 0x0010)); entry != nil {
		t.Fatalf("private Tag unexpectedly has dictionary VRs: %#v", entry.VRs())
	}
}

func TestGoDicomVRParsingRejectsUnknownVR(t *testing.T) {
	parsed, err := vr.Parse("LO")
	if err != nil || parsed.Code() != "LO" {
		t.Fatalf("vr.Parse(LO) = %#v, %v", parsed, err)
	}
	if _, ok := vr.TryParse("ZZ"); ok {
		t.Fatal("vr.TryParse(ZZ) succeeded")
	}
}

func TestGoDicomUIDGenerationAndMapping(t *testing.T) {
	derived := uid.GenerateDerivedFromUUID().UID()
	if !strings.HasPrefix(derived, "2.25.") || strings.TrimPrefix(derived, "2.25.") == "" {
		t.Fatalf("UUID-derived UID = %q, want 2.25.<decimal>", derived)
	}
	for _, value := range strings.TrimPrefix(derived, "2.25.") {
		if value < '0' || value > '9' {
			t.Fatalf("UUID-derived UID suffix = %q, want decimal digits", derived)
		}
	}
	if another := uid.GenerateDerivedFromUUID().UID(); another == derived {
		t.Fatalf("two UUID-derived UIDs are identical: %q", derived)
	}
	if got := uid.GenerateFromRoot("1.2.826.0.1.3680043.10.5432", 42).UID(); got != "1.2.826.0.1.3680043.10.5432.42" {
		t.Fatalf("root-derived UID = %q", got)
	}

	generator := uid.NewGenerator()
	study := uid.New("1.2.826.0.1.3680043.10.5432.1", "Study", uid.TypeUnknown, false)
	series := uid.New("1.2.826.0.1.3680043.10.5432.2", "Series", uid.TypeUnknown, false)
	sop := uid.New("1.2.826.0.1.3680043.10.5432.3", "SOP", uid.TypeUnknown, false)
	referencedSOP := uid.New("1.2.826.0.1.3680043.10.5432.3", "Referenced SOP", uid.TypeUnknown, false)
	first := generator.Generate(study).UID()
	if got := generator.Generate(study).UID(); got != first {
		t.Fatalf("same source UID mapped to %q after %q", got, first)
	}
	if got := generator.Generate(series).UID(); got == first {
		t.Fatalf("different source UIDs mapped to the same UID %q", got)
	}
	if generator.Generate(sop).UID() != generator.Generate(referencedSOP).UID() {
		t.Fatal("SOP Instance UID and its reference have different mappings")
	}
}
