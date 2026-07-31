package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	editpkg "github.com/cocosip/dicom-cli/internal/edit"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
)

func newEditCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	var sets, clears, deletes, generates, vrs []string
	var destination, uidRoot, charset, inputCharset string
	var remapUIDs bool
	command := &cobra.Command{
		Use:   "edit <file>",
		Short: "Edit one DICOM file into a new file",
		Long:  "Apply tag edits to one DICOM file and always write a new output file. At least one edit operation is required. Private or unknown tags require --vr TagPath=VR when their VR cannot be inferred.",
		Example: "  dicom-cli edit image.dcm --set PatientName=ANON^PATIENT --output edited.dcm\n" +
			"  dicom-cli edit image.dcm --clear PatientID --output edited.dcm",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if root.rulesPath != "" {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("edit does not accept a rules profile"))
			}
			if err := requireRegularFile(args[0]); err != nil {
				return err
			}
			outputPath, err := newOutputPath(runtime, args[0], destination)
			if err != nil {
				return err
			}
			operations, err := parseEditOperations(sets, clears, deletes, generates, vrs)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if len(operations) == 0 {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("at least one edit operation is required"))
			}
			parsed, err := parser.ParseFile(args[0])
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			if err := editpkg.Apply(parsed.Dataset, operations, editpkg.Options{UIDRoot: uidRoot, RemapUIDs: remapUIDs}); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if charset != "" || inputCharset != "" {
				outputCharset := charset
				if outputCharset == "" {
					outputCharset = inputCharset
				}
				if err := editpkg.ConvertCharacterSet(parsed.Dataset, outputCharset, inputCharset); err != nil {
					return apperr.Wrap(apperr.KindInput, err)
				}
			}
			if err := writer.WriteFile(outputPath, parsed.Dataset, writer.WithTransferSyntax(parsed.TransferSyntax)); err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			_, err = fmt.Fprintln(runtime.Stdout, outputPath)
			return err
		},
	}
	command.Flags().StringArrayVar(&sets, "set", nil, "TagPath=value")
	command.Flags().StringArrayVar(&clears, "clear", nil, "TagPath")
	command.Flags().StringArrayVar(&deletes, "delete", nil, "TagPath")
	command.Flags().StringArrayVar(&generates, "generate-uid", nil, "UID TagPath")
	command.Flags().StringArrayVar(&vrs, "vr", nil, "TagPath=VR for private or unknown Tags")
	command.Flags().StringVarP(&destination, "output", "o", "", "new DICOM output path")
	command.Flags().StringVar(&uidRoot, "uid-root", "", "UID root for generated UIDs")
	command.Flags().BoolVar(&remapUIDs, "remap-uids", false, "remap all UID values in the file")
	command.Flags().StringVar(&charset, "charset", "", "output character set")
	command.Flags().StringVar(&inputCharset, "input-charset", "", "override input character set")
	return command
}

func parseEditOperations(sets, clears, deletes, generates, vrs []string) ([]editpkg.Operation, error) {
	vrByPath := make(map[string]string, len(vrs))
	for _, entry := range vrs {
		path, value, ok := strings.Cut(entry, "=")
		if !ok || path == "" || value == "" {
			return nil, fmt.Errorf("--vr must be TagPath=VR")
		}
		vrByPath[path] = value
	}
	operations := make([]editpkg.Operation, 0, len(sets)+len(clears)+len(deletes)+len(generates))
	for _, set := range sets {
		path, value, ok := strings.Cut(set, "=")
		if !ok || path == "" {
			return nil, fmt.Errorf("--set must be TagPath=value")
		}
		operations = append(operations, editpkg.Operation{Kind: editpkg.Set, Path: path, Value: value, VR: vrByPath[path]})
	}
	for _, path := range clears {
		if path == "" {
			return nil, fmt.Errorf("--clear requires a TagPath")
		}
		operations = append(operations, editpkg.Operation{Kind: editpkg.Clear, Path: path})
	}
	for _, path := range deletes {
		if path == "" {
			return nil, fmt.Errorf("--delete requires a TagPath")
		}
		operations = append(operations, editpkg.Operation{Kind: editpkg.Delete, Path: path})
	}
	for _, path := range generates {
		if path == "" {
			return nil, fmt.Errorf("--generate-uid requires a TagPath")
		}
		operations = append(operations, editpkg.Operation{Kind: editpkg.GenerateUID, Path: path})
	}
	return operations, nil
}
