package main

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func TestRuntimeRegistersCompressedCodecs(t *testing.T) {
	registry := codec.GetGlobalRegistry()

	for _, syntax := range []*transfer.Syntax{
		transfer.RLELossless,
		transfer.JPEGBaseline8Bit,
		transfer.JPEGExtended12Bit,
		transfer.JPEGLossless,
		transfer.JPEGLosslessSV1,
		transfer.JPEGLSLossless,
		transfer.JPEGLSNearLossless,
		transfer.JPEG2000Lossless,
		transfer.JPEG2000Lossy,
		transfer.JPEG2000Part2MultiComponentLosslessOnly,
		transfer.JPEG2000Part2MultiComponent,
		transfer.HTJ2KLossless,
		transfer.HTJ2KLosslessRPCL,
		transfer.HTJ2K,
	} {
		if !registry.HasCodec(syntax) {
			t.Errorf("codec for transfer syntax %s is not registered", syntax.UID().UID())
		}
	}
}
