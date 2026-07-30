package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/rules"
	validatepkg "github.com/cocosip/dicom-cli/internal/validate"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

func newValidateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	var profile string
	var strict bool
	var asJSON bool
	var destination string
	command := &cobra.Command{
		Use: "validate <file>", Short: "Validate a single DICOM file", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := requireRegularFile(args[0]); err != nil {
				return err
			}
			var profiles []rules.ValidateProfile
			if profile != "" {
				path, err := rulesPath(runtime, root.rulesPath, nil)
				if err != nil {
					return apperr.Wrap(apperr.KindInput, err)
				}
				file, err := rules.Load(path)
				if err != nil {
					return apperr.Wrap(apperr.KindInput, err)
				}
				selected, ok := file.ValidateRules.Profiles[profile]
				if !ok {
					return apperr.Wrap(apperr.KindInput, fmt.Errorf("validate profile %q does not exist", profile))
				}
				profiles = append(profiles, selected)
			}
			parsed, err := parser.ParseFile(args[0])
			if err != nil {
				return apperr.Wrap(apperr.KindOperation, err)
			}
			result := validatepkg.Validate(parsed, profiles...)
			content, err := renderValidate(result, asJSON)
			if err != nil {
				return err
			}
			if err := writeReport(runtime, args[0], destination, content); err != nil {
				return err
			}
			return result.Failure(strict)
		},
	}
	command.Flags().StringVarP(&profile, "profile", "p", "", "validate profile from rules")
	command.Flags().BoolVar(&strict, "strict", false, "treat warnings as failures")
	command.Flags().BoolVarP(&asJSON, "json", "j", false, "write JSON")
	command.Flags().StringVarP(&destination, "output", "o", "", "report output path")
	return command
}

func renderValidate(result validatepkg.Result, asJSON bool) ([]byte, error) {
	if asJSON {
		return json.MarshalIndent(result, "", "  ")
	}
	if len(result.Issues) == 0 {
		return []byte("valid\n"), nil
	}
	lines := make([]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		lines = append(lines, fmt.Sprintf("%s %s %s: %s", issue.Severity, issue.Source, issue.Path, issue.Message))
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}
