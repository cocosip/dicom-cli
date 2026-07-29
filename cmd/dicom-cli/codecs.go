package main

import (
	_ "github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/extended"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codecs/jpegls/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpegls/nearlossless"
	_ "github.com/cocosip/go-dicom-codecs/rle"
)
