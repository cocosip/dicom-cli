# Project Instructions

## Go DICOM Implementations

- For DICOM or DIMSE work in Go, first inspect and reuse the capabilities already
  provided by `github.com/cocosip/go-dicom`. This includes parsing, encoding,
  PDU, Association, and SCU behavior where the library supports them.
- Do not design or implement a parallel DICOM/DIMSE protocol stack before
  verifying that `go-dicom` lacks the required capability. Keep any adapter
  narrowly scoped to the missing boundary.
