package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/dicom"
)

func newTranscodeCommand(runtime Runtime, _ *rootOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "transcode",
		Short: "Re-encode DICOM transfer syntaxes",
	}
	var asJSON bool
	formats := &cobra.Command{
		Use:   "formats",
		Short: "List transfer syntaxes available in this binary",
		Args:  noArgs,
		RunE: func(*cobra.Command, []string) error {
			available := dicom.RuntimeCodecs()
			if asJSON {
				return json.NewEncoder(runtime.Stdout).Encode(available)
			}
			for _, format := range available {
				marker := ""
				if format.Experimental {
					marker = " experimental"
				}
				if _, err := fmt.Fprintf(runtime.Stdout, "%s\t%s\tencode=%t\tdecode=%t%s\n", format.Alias, format.UID, format.Encode, format.Decode, marker); err != nil {
					return err
				}
			}
			return nil
		},
	}
	formats.Flags().BoolVarP(&asJSON, "json", "j", false, "write JSON output")
	command.AddCommand(formats)
	return command
}
