package main

import "github.com/spf13/cobra"

func newEncapsulateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "encapsulate",
		Short: "Encapsulate external content as DICOM",
		Long:  "External images are imported into uncompressed Secondary Capture DICOM files. Select the image subcommand to provide metadata and output handling.",
		Args:  noArgs,
	}

	var patientName, templateName, referencePath, destination string
	var recursive, failFast, flatten bool
	imageCommand := &cobra.Command{
		Use:   "image <input>",
		Short: "Encapsulate PNG or JPEG images as Secondary Capture DICOM",
		Long:  "Encapsulate supported PNG or JPEG images as uncompressed Explicit VR Little Endian Secondary Capture DICOM. PatientName must come from --patient-name, --template, or --reference. For directory input, Study and Series UIDs are shared while each image receives a distinct SOP Instance UID.",
		Example: "  dicom-cli encapsulate image --patient-name ANON^PATIENT --output image.dcm source.png\n" +
			"  dicom-cli encapsulate image --template secondary-capture --rules dicom-cli-rules.yaml --output output images",
		Args: cobra.ExactArgs(1),
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
