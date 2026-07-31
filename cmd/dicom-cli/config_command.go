package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/config"
)

func newConfigCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Manage runtime configuration"}
	command.AddCommand(newConfigInitCommand(runtime))
	command.AddCommand(newConfigValidateCommand(runtime, root))
	command.AddCommand(newConfigTargetCommand(runtime, root))
	return command
}

func newConfigInitCommand(runtime Runtime) *cobra.Command {
	var format string
	var force bool
	command := &cobra.Command{
		Use:   "init [path]",
		Short: "Create a runtime configuration example",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := initPath(runtime, args, format)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if !force {
				if _, err := os.Stat(path); err == nil {
					return apperr.Wrap(apperr.KindInput, fmt.Errorf("configuration file %q already exists; use --force to overwrite", path))
				} else if !os.IsNotExist(err) {
					return apperr.Wrap(apperr.KindInput, err)
				}
			}
			if err := config.Write(path, config.DefaultExampleConfig(), format); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			_, err = fmt.Fprintln(runtime.Stdout, path)
			return err
		},
	}
	command.Flags().StringVar(&format, "format", "yaml", "configuration format: yaml or json")
	command.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	return command
}

func newConfigValidateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a runtime configuration",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := loadOptions(runtime, root.configPath, args)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			_, _, err = config.Load(options)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			_, err = fmt.Fprintln(runtime.Stdout, "valid")
			return err
		},
	}
}

func newConfigTargetCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "target", Short: "Manage named PACS targets"}
	command.AddCommand(newTargetListCommand(runtime, root))
	command.AddCommand(newTargetAddCommand(runtime, root))
	command.AddCommand(newTargetUpdateCommand(runtime, root))
	command.AddCommand(newTargetRemoveCommand(runtime, root))
	return command
}

func newTargetListCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List named PACS targets",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			loaded, _, err := loadMutableConfig(runtime, root)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			names := make([]string, 0, len(loaded.Targets))
			for name := range loaded.Targets {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				if _, err := fmt.Fprintln(runtime.Stdout, name); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newTargetAddCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	values := targetFlagValues{}
	command := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a named PACS target",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			loaded, location, err := loadMutableConfig(runtime, root)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if _, exists := loaded.Targets[args[0]]; exists {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("target %q already exists", args[0]))
			}
			if err := values.requireAll(command); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			loaded.Targets[args[0]] = values.target(config.PACSTarget{})
			if err := config.Write(location.Path, loaded, ""); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			return nil
		},
	}
	values.addFlags(command)
	return command
}

func newTargetUpdateCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	values := targetFlagValues{}
	command := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a named PACS target",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			loaded, location, err := loadMutableConfig(runtime, root)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			target, exists := loaded.Targets[args[0]]
			if !exists {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("target %q does not exist", args[0]))
			}
			loaded.Targets[args[0]] = values.targetWhenChanged(command, target)
			if err := config.Write(location.Path, loaded, ""); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			return nil
		},
	}
	values.addFlags(command)
	return command
}

func newTargetRemoveCommand(runtime Runtime, root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a named PACS target",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			loaded, location, err := loadMutableConfig(runtime, root)
			if err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			if _, exists := loaded.Targets[args[0]]; !exists {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("target %q does not exist", args[0]))
			}
			delete(loaded.Targets, args[0])
			if err := config.Write(location.Path, loaded, ""); err != nil {
				return apperr.Wrap(apperr.KindInput, err)
			}
			return nil
		},
	}
}

type targetFlagValues struct {
	host      string
	port      int
	callingAE string
	calledAE  string
}

func (values *targetFlagValues) addFlags(command *cobra.Command) {
	command.Flags().StringVar(&values.host, "host", "", "PACS host")
	command.Flags().IntVar(&values.port, "port", 0, "PACS port")
	command.Flags().StringVar(&values.callingAE, "calling-ae", "", "calling AE Title")
	command.Flags().StringVar(&values.calledAE, "called-ae", "", "called AE Title")
}

func (values targetFlagValues) requireAll(command *cobra.Command) error {
	for _, name := range []string{"host", "port", "calling-ae", "called-ae"} {
		if !command.Flags().Changed(name) {
			return fmt.Errorf("--%s is required", name)
		}
	}
	return nil
}

func (values targetFlagValues) target(existing config.PACSTarget) config.PACSTarget {
	existing.Host = values.host
	existing.Port = values.port
	existing.CallingAETitle = values.callingAE
	existing.CalledAETitle = values.calledAE
	return existing
}

func (values targetFlagValues) targetWhenChanged(command *cobra.Command, existing config.PACSTarget) config.PACSTarget {
	if command.Flags().Changed("host") {
		existing.Host = values.host
	}
	if command.Flags().Changed("port") {
		existing.Port = values.port
	}
	if command.Flags().Changed("calling-ae") {
		existing.CallingAETitle = values.callingAE
	}
	if command.Flags().Changed("called-ae") {
		existing.CalledAETitle = values.calledAE
	}
	return existing
}

func initPath(runtime Runtime, args []string, format string) (string, error) {
	if format != "yaml" && format != "json" {
		return "", fmt.Errorf("unsupported configuration format %q", format)
	}
	if len(args) == 1 {
		return args[0], nil
	}
	workingDir, err := runtime.Getwd()
	if err != nil {
		return "", err
	}
	name := config.DefaultFileName
	if format == "json" {
		name = "dicom-cli.json"
	}
	return filepath.Join(workingDir, name), nil
}

func loadOptions(runtime Runtime, configuredPath string, args []string) (config.LocateOptions, error) {
	if runtime.UserConfigDir == nil {
		return config.LocateOptions{}, fmt.Errorf("user configuration directory is unavailable")
	}
	workingDir, err := runtime.Getwd()
	if err != nil {
		return config.LocateOptions{}, err
	}
	userConfigDir, err := runtime.UserConfigDir()
	if err != nil {
		return config.LocateOptions{}, err
	}
	path := configuredPath
	if len(args) == 1 {
		path = args[0]
	}
	return config.LocateOptions{Path: path, WorkingDir: workingDir, UserConfigDir: userConfigDir, LookupEnv: runtime.LookupEnv}, nil
}

func loadMutableConfig(runtime Runtime, root *rootOptions) (config.Config, config.Location, error) {
	options, err := loadOptions(runtime, root.configPath, nil)
	if err != nil {
		return config.Config{}, config.Location{}, err
	}
	loaded, location, err := config.Load(options)
	if err != nil {
		return config.Config{}, config.Location{}, err
	}
	if location.Path == "" {
		return config.Config{}, config.Location{}, fmt.Errorf("target management requires an existing configuration file")
	}
	return loaded, location, nil
}
