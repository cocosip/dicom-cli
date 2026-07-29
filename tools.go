//go:build tools

package tools

import (
	_ "github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	_ "github.com/cocosip/go-dicom/pkg/dicom/parser"
	_ "github.com/spf13/cobra"
	_ "github.com/spf13/viper"
)
