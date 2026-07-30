// Package testutil provides synthetic DICOM fixtures for internal tests.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

// DICOMFixtures contains paths to synthetic, non-identifying DICOM files.
type DICOMFixtures struct {
	SingleFrame  string
	MultiFrame   string
	Sequence     string
	UIDReference string
	Corrupt      string
}

// CreateDICOMFixtures writes a reusable fixture set under directory.
func CreateDICOMFixtures(directory string) (DICOMFixtures, error) {
	fixtures := DICOMFixtures{
		SingleFrame:  filepath.Join(directory, "single-frame.dcm"),
		MultiFrame:   filepath.Join(directory, "multi-frame.dcm"),
		Sequence:     filepath.Join(directory, "sequence.dcm"),
		UIDReference: filepath.Join(directory, "uid-reference.dcm"),
		Corrupt:      filepath.Join(directory, "corrupt.dcm"),
	}

	if err := writeFixture(fixtures.SingleFrame, baseDataset("1.2.826.0.1.3680043.10.5432.1")); err != nil {
		return DICOMFixtures{}, err
	}

	multiFrame := baseDataset("1.2.826.0.1.3680043.10.5432.2")
	if err := multiFrame.Add(element.NewString(tag.NumberOfFrames, vr.IS, []string{"2"})); err != nil {
		return DICOMFixtures{}, err
	}
	if err := multiFrame.AddOrUpdate(element.NewOtherWord(tag.PixelData, []byte{0, 0, 1, 0, 2, 0, 3, 0})); err != nil {
		return DICOMFixtures{}, err
	}
	if err := writeFixture(fixtures.MultiFrame, multiFrame); err != nil {
		return DICOMFixtures{}, err
	}

	withSequence := baseDataset("1.2.826.0.1.3680043.10.5432.3")
	item := dataset.New()
	if err := item.Add(element.NewString(tag.TextValue, vr.UT, []string{"synthetic nested value"})); err != nil {
		return DICOMFixtures{}, err
	}
	if err := withSequence.Add(dataset.NewSequenceWithItems(tag.ContentSequence, []*dataset.Dataset{item})); err != nil {
		return DICOMFixtures{}, err
	}
	if err := writeFixture(fixtures.Sequence, withSequence); err != nil {
		return DICOMFixtures{}, err
	}

	withReference := baseDataset("1.2.826.0.1.3680043.10.5432.4")
	if err := withReference.Add(element.NewString(tag.ReferencedSOPInstanceUID, vr.UI, []string{"1.2.826.0.1.3680043.10.5432.1"})); err != nil {
		return DICOMFixtures{}, err
	}
	if err := writeFixture(fixtures.UIDReference, withReference); err != nil {
		return DICOMFixtures{}, err
	}
	if err := os.WriteFile(fixtures.Corrupt, []byte("not a DICOM file"), 0o600); err != nil {
		return DICOMFixtures{}, fmt.Errorf("write corrupt fixture: %w", err)
	}

	return fixtures, nil
}

func baseDataset(sopInstanceUID string) *dataset.Dataset {
	dataset := dataset.New()
	for _, element := range []element.Element{
		element.NewString(tag.PatientName, vr.PN, []string{"SYNTHETIC^PATIENT"}),
		element.NewString(tag.PatientID, vr.LO, []string{"SYNTHETIC"}),
		element.NewString(tag.PatientBirthDate, vr.DA, []string{"19800102"}),
		element.NewString(tag.PatientSex, vr.CS, []string{"F"}),
		element.NewString(tag.StudyInstanceUID, vr.UI, []string{"1.2.826.0.1.3680043.10.5432"}),
		element.NewString(tag.StudyDate, vr.DA, []string{"20260730"}),
		element.NewString(tag.StudyTime, vr.TM, []string{"123456"}),
		element.NewString(tag.AccessionNumber, vr.SH, []string{"ACC-001"}),
		element.NewString(tag.ReferringPhysicianName, vr.PN, []string{"SYNTHETIC^REFERRER"}),
		element.NewString(tag.StudyDescription, vr.LO, []string{"Synthetic CT study"}),
		element.NewString(tag.SeriesInstanceUID, vr.UI, []string{"1.2.826.0.1.3680043.10.5432.1"}),
		element.NewString(tag.SeriesNumber, vr.IS, []string{"7"}),
		element.NewString(tag.SeriesDescription, vr.LO, []string{"Synthetic axial"}),
		element.NewString(tag.BodyPartExamined, vr.CS, []string{"CHEST"}),
		element.NewString(tag.Laterality, vr.CS, []string{"R"}),
		element.NewString(tag.ProtocolName, vr.LO, []string{"Chest routine"}),
		element.NewString(tag.SOPClassUID, vr.UI, []string{"1.2.840.10008.5.1.4.1.1.2"}),
		element.NewString(tag.SOPInstanceUID, vr.UI, []string{sopInstanceUID}),
		element.NewString(tag.InstanceNumber, vr.IS, []string{"42"}),
		element.NewString(tag.ImagePositionPatient, vr.DS, []string{"0", "0", "-10"}),
		element.NewString(tag.ImageOrientationPatient, vr.DS, []string{"1", "0", "0", "0", "1", "0"}),
		element.NewString(tag.SliceThickness, vr.DS, []string{"1.5"}),
		element.NewString(tag.SpacingBetweenSlices, vr.DS, []string{"1.2"}),
		element.NewString(tag.Modality, vr.CS, []string{"CT"}),
		element.NewUnsignedShort(tag.Rows, []uint16{1}),
		element.NewUnsignedShort(tag.Columns, []uint16{2}),
		element.NewUnsignedShort(tag.SamplesPerPixel, []uint16{1}),
		element.NewString(tag.PhotometricInterpretation, vr.CS, []string{"MONOCHROME2"}),
		element.NewString(tag.PixelSpacing, vr.DS, []string{"0.5", "0.5"}),
		element.NewUnsignedShort(tag.BitsAllocated, []uint16{16}),
		element.NewUnsignedShort(tag.BitsStored, []uint16{16}),
		element.NewUnsignedShort(tag.HighBit, []uint16{15}),
		element.NewUnsignedShort(tag.PixelRepresentation, []uint16{0}),
		element.NewString(tag.WindowCenter, vr.DS, []string{"40"}),
		element.NewString(tag.WindowWidth, vr.DS, []string{"400"}),
		element.NewOtherWord(tag.PixelData, []byte{0, 0, 1, 0}),
	} {
		_ = dataset.Add(element)
	}
	return dataset
}

func writeFixture(path string, dataset *dataset.Dataset) error {
	if err := writer.WriteFile(path, dataset, writer.WithTransferSyntax(transfer.ExplicitVRLittleEndian)); err != nil {
		return fmt.Errorf("write fixture %q: %w", path, err)
	}
	return nil
}
