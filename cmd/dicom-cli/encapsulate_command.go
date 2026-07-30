package main

import "github.com/spf13/cobra"

func newEncapsulateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "encapsulate",
		Short: "Encapsulate external content as DICOM",
		Args:  noArgs,
	}

	var patientName, templateName, referencePath, destination string
	var recursive, failFast, flatten bool
	imageCommand := &cobra.Command{
		Use:   "image <input>",
		Short: "Encapsulate PNG or JPEG images as Secondary Capture DICOM",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runImageToDICOMWithMetadata(runtime, root, args[0], patientName, templateName, referencePath, destination, recursive, failFast, flatten)
		},
	}
	imageCommand.Flags().StringVar(&patientName, "patient-name", "", "required PatientName for created DICOM files")
	imageCommand.Flags().StringVar(&templateName, "template", "", "named DICOM template from rules")
	imageCommand.Flags().StringVar(&referencePath, "reference", "", "reference DICOM metadata source")
	imageCommand.Flags().StringVarP(&destination, "output", "o", "", "DICOM output file or directory")
	imageCommand.Flags().BoolVarP(&recursive, "recursive", "r", false, "scan subdirectories")
	imageCommand.Flags().BoolVar(&failFast, "fail-fast", false, "stop after the first file failure")
	imageCommand.Flags().BoolVar(&flatten, "flatten", false, "do not preserve input directory structure")

	command.AddCommand(imageCommand)
	return command
}
