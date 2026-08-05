package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/i18n"
	"github.com/cocosip/dicom-cli/internal/rules"
	validatepkg "github.com/cocosip/dicom-cli/internal/validate"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
)

func newValidateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("validate")
	var profile string
	var strict bool
	var charsetCheck bool
	var asJSON bool
	var destination string
	command := &cobra.Command{
		Use:   "validate <file>",
		Short: text.Short,
		Long:  text.Long,
		Example: "  dicom-cli validate image.dcm\n" +
			"  dicom-cli validate --strict --json --output validation.json image.dcm",
		Args: cobra.ExactArgs(1),
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
			if charsetCheck {
				result.Issues = append(result.Issues, validatepkg.CheckCharacterSet(parsed.Dataset)...)
			}
			content, err := renderValidate(result, asJSON, root.localizer)
			if err != nil {
				return err
			}
			if err := writeReport(runtime, args[0], destination, content); err != nil {
				return err
			}
			return result.Failure(strict)
		},
	}
	localizedHelpFlag(command, root.localizer)
	command.Flags().StringVarP(&profile, "profile", "p", "", root.localizer.FlagUsage("profile", "validate profile from rules"))
	command.Flags().BoolVar(&strict, "strict", false, root.localizer.FlagUsage("strict", "treat warnings as failures"))
	command.Flags().BoolVar(&charsetCheck, "charset-check", false, root.localizer.FlagUsage("charset-check", "detect mismatches between Specific Character Set and raw text bytes"))
	command.Flags().BoolVarP(&asJSON, "json", "j", false, root.localizer.FlagUsage("json", "write JSON"))
	command.Flags().StringVarP(&destination, "output", "o", "", root.localizer.FlagUsage("output", "report output path"))
	return command
}

func renderValidate(result validatepkg.Result, asJSON bool, localizer i18n.Localizer) ([]byte, error) {
	if asJSON {
		return json.MarshalIndent(result, "", "  ")
	}
	if len(result.Issues) == 0 {
		return []byte(localizer.Text(i18n.ValidationValid) + "\n"), nil
	}
	lines := make([]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		lines = append(lines, fmt.Sprintf("%s %s %s: %s", issue.Severity, issue.Source, issue.Path, issue.Message))
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}
