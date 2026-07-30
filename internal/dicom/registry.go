package dicom

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// CodecFormat describes a transfer syntax that the running binary can use.
type CodecFormat struct {
	Alias        string           `json:"alias"`
	UID          string           `json:"uid"`
	Encode       bool             `json:"encode"`
	Decode       bool             `json:"decode"`
	Experimental bool             `json:"experimental,omitempty"`
	Syntax       *transfer.Syntax `json:"-"`
}

type formatMetadata struct {
	alias        string
	experimental bool
}

var formatMetadataByUID = map[string]formatMetadata{
	"1.2.840.10008.1.2":       {alias: "implicit-vr-little-endian"},
	"1.2.840.10008.1.2.1":     {alias: "explicit-vr-little-endian"},
	"1.2.840.10008.1.2.2":     {alias: "explicit-vr-big-endian"},
	"1.2.840.10008.1.2.5":     {alias: "rle"},
	"1.2.840.10008.1.2.4.50":  {alias: "jpeg-baseline"},
	"1.2.840.10008.1.2.4.51":  {alias: "jpeg-extended"},
	"1.2.840.10008.1.2.4.57":  {alias: "jpeg-lossless"},
	"1.2.840.10008.1.2.4.70":  {alias: "jpeg-lossless-sv1"},
	"1.2.840.10008.1.2.4.80":  {alias: "jpeg-ls"},
	"1.2.840.10008.1.2.4.81":  {alias: "jpeg-ls-near-lossless"},
	"1.2.840.10008.1.2.4.90":  {alias: "jpeg2000-lossless"},
	"1.2.840.10008.1.2.4.91":  {alias: "jpeg2000"},
	"1.2.840.10008.1.2.4.92":  {alias: "jpeg2000-multicomponent-lossless"},
	"1.2.840.10008.1.2.4.93":  {alias: "jpeg2000-multicomponent"},
	"1.2.840.10008.1.2.4.201": {alias: "htj2k-lossless", experimental: true},
	"1.2.840.10008.1.2.4.202": {alias: "htj2k-lossless-rpcl", experimental: true},
	"1.2.840.10008.1.2.4.203": {alias: "htj2k", experimental: true},
}

// RuntimeCodecs lists only transfer syntaxes that have a codec registered in
// the current process. Metadata supplies user-facing aliases but never adds
// capabilities that the registry did not report.
func RuntimeCodecs() []CodecFormat {
	registry := codec.GetGlobalRegistry()
	formats := make([]CodecFormat, 0)
	for _, uid := range registry.ListCodecs() {
		syntax, err := transfer.Parse(uid)
		if err != nil {
			continue
		}
		metadata := formatMetadataByUID[uid]
		alias := metadata.alias
		if alias == "" {
			alias = uid
		}
		formats = append(formats, CodecFormat{
			Alias:        alias,
			UID:          uid,
			Encode:       true,
			Decode:       true,
			Experimental: metadata.experimental,
			Syntax:       syntax,
		})
	}
	sort.Slice(formats, func(i, j int) bool { return formats[i].Alias < formats[j].Alias })
	return formats
}

// ResolveTransferSyntax looks up an available transfer syntax by alias or UID.
func ResolveTransferSyntax(value string) (CodecFormat, error) {
	needle := strings.ToLower(strings.TrimSpace(value))
	for _, format := range RuntimeCodecs() {
		if needle == strings.ToLower(format.Alias) || needle == format.UID {
			return format, nil
		}
	}
	return CodecFormat{}, fmt.Errorf("transfer syntax %q is not available in this binary", value)
}
