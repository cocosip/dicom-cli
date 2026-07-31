package main

import "github.com/spf13/cobra"

func newEncapsulateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("encapsulate")
	command := &cobra.Command{
		Use:   "encapsulate",
		Short: text.Short,
		Long:  text.Long,
		Args:  noArgs,
	}

	var patientName, templateName, referencePath, destination string
	var recursive, failFast, flatten bool
	imageText := root.localizer.Command("encapsulate image")
	imageCommand := &cobra.Command{
		Use:   "image <input>",
		Short: imageText.Short,
		Long:  imageText.Long,
		Example: "  dicom-cli encapsulate image --patient-name ANON^PATIENT --output image.dcm source.png\n" +
			"  dicom-cli encapsulate image --template secondary-capture --rules dicom-cli-rules.yaml --output output images",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runImageToDICOMWithMetadata(runtime, root, args[0], patientName, templateName, referencePath, destination, recursive, failFast, flatten)
		},
	}
	imageCommand.Flags().StringVar(&patientName, "patient-name", "", root.localizer.FlagUsage("patient-name", "required PatientName for created DICOM files"))
	imageCommand.Flags().StringVar(&templateName, "template", "", root.localizer.FlagUsage("template", "named DICOM template from rules"))
	imageCommand.Flags().StringVar(&referencePath, "reference", "", root.localizer.FlagUsage("reference", "reference DICOM metadata source"))
	imageCommand.Flags().StringVarP(&destination, "output", "o", "", root.localizer.FlagUsage("output", "DICOM output file or directory"))
	imageCommand.Flags().BoolVarP(&recursive, "recursive", "r", false, root.localizer.FlagUsage("recursive", "scan subdirectories"))
	imageCommand.Flags().BoolVar(&failFast, "fail-fast", false, root.localizer.FlagUsage("fail-fast", "stop after the first file failure"))
	imageCommand.Flags().BoolVar(&flatten, "flatten", false, root.localizer.FlagUsage("flatten", "do not preserve input directory structure"))

	localizedHelpFlag(command, root.localizer)
	localizedHelpFlag(imageCommand, root.localizer)
	command.AddCommand(imageCommand)
	return command
}
