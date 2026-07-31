package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/i18n"
	"github.com/cocosip/dicom-cli/internal/rules"
)

func newRulesCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("rules")
	command := &cobra.Command{
		Use:   "rules",
		Short: text.Short,
		Long:  text.Long,
	}
	localizedHelpFlag(command, root.localizer)
	command.AddCommand(newRulesInitCommand(runtime, root))
	command.AddCommand(newRulesValidateCommand(runtime, root))
	return command
}

func newRulesInitCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("rules init")
	var format string
	var force bool
	command := &cobra.Command{
		Use:   "init [path]",
		Short: text.Short,
		Long:  text.Long,
		Example: "  dicom-cli rules init dicom-cli-rules.yaml\n" +
			"  dicom-cli rules init dicom-cli-rules.json --format json",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := rulesInitPath(runtime, args, format)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if !force {
				if _, err := os.Stat(path); err == nil {
					return apperr.Wrap(apperr.KindInput, fmt.Errorf("rules file %q already exists; use --force to overwrite", path))
				} else if !os.IsNotExist(err) {
					return apperr.Wrap(apperr.KindInput, err)
				}
			}
			content, err := rules.Example(format)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if err := os.WriteFile(path, content, 0o600); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			_, err = fmt.Fprintln(runtime.Stdout, path)
			return err
		},
	}
	localizedHelpFlag(command, root.localizer)
	command.Flags().StringVar(&format, "format", "yaml", root.localizer.FlagUsage("format", "rules format: yaml or json"))
	command.Flags().BoolVar(&force, "force", false, root.localizer.FlagUsage("force", "overwrite an existing file"))
	return command
}

func newRulesValidateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	text := root.localizer.Command("rules validate")
	command := &cobra.Command{
		Use:     "validate [path]",
		Short:   text.Short,
		Long:    text.Long,
		Example: "  dicom-cli rules validate dicom-cli-rules.yaml",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := rulesPath(runtime, root.rulesPath, args)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if _, err := rules.Load(path); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			_, err = fmt.Fprintln(runtime.Stdout, root.localizer.Text(i18n.ValidationValid))
			return err
		},
	}
	localizedHelpFlag(command, root.localizer)
	return command
}

func rulesInitPath(runtime Runtime, args []string, format string) (string, error) {
	if format != "yaml" && format != "json" {
		return "", fmt.Errorf("unsupported rules format %q", format)
	}
	if len(args) == 1 {
		return args[0], nil
	}
	workingDir, err := runtime.Getwd()
	if err != nil {
		return "", err
	}
	name := rules.DefaultFileName
	if format == "json" {
		name = "dicom-cli-rules.json"
	}
	return filepath.Join(workingDir, name), nil
}

func rulesPath(runtime Runtime, configuredPath string, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if configuredPath != "" {
		return configuredPath, nil
	}
	if path, ok := runtime.LookupEnv("DICOM_CLI_RULES"); ok && path != "" {
		return path, nil
	}
	workingDir, err := runtime.Getwd()
	if err != nil {
		return "", err
	}
	path := filepath.Join(workingDir, rules.DefaultFileName)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if runtime.UserConfigDir == nil {
		return "", fmt.Errorf("rules file was not found")
	}
	userDir, err := runtime.UserConfigDir()
	if err != nil {
		return "", err
	}
	path = filepath.Join(userDir, rules.DefaultFileName)
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}
