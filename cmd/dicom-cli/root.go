package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/logging"
)

// Runtime provides process dependencies to commands without requiring globals.
type Runtime struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Getwd     func() (string, error)
	LookupEnv func(string) (string, bool)
}

// ProductionRuntime returns the dependencies used by the executable.
func ProductionRuntime() Runtime {
	return Runtime{
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getwd:     os.Getwd,
		LookupEnv: os.LookupEnv,
	}
}

type rootOptions struct {
	configPath string
	rulesPath  string
	verbose    bool
	quiet      bool
	logFormat  string
}

// Execute runs the root command and returns the process exit code.
func Execute(args []string, runtime Runtime) int {
	command := NewRootCommand(runtime)
	command.SetArgs(args)

	if err := command.Execute(); err != nil {
		fmt.Fprintln(runtime.Stderr, err)
		return apperr.ExitCode(err)
	}

	return 0
}

// NewRootCommand builds the root Cobra command from injected process dependencies.
func NewRootCommand(runtime Runtime) *cobra.Command {
	options := rootOptions{logFormat: "text"}

	command := &cobra.Command{
		Use:           "dicom-cli",
		Short:         "DICOM command-line utility",
		Args:          noArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			if options.verbose && options.quiet {
				return apperr.Wrap(apperr.KindInput, fmt.Errorf("--verbose and --quiet cannot be used together"))
			}

			_, err := logging.New(logLevel(options), options.logFormat, runtime.Stderr)
			return err
		},
	}

	command.SetIn(runtime.Stdin)
	command.SetOut(runtime.Stdout)
	command.SetErr(runtime.Stderr)
	command.SetFlagErrorFunc(func(*cobra.Command, error) error {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("invalid command arguments"))
	})

	flags := command.PersistentFlags()
	flags.StringVarP(&options.configPath, "config", "c", "", "configuration file")
	flags.StringVarP(&options.rulesPath, "rules", "R", "", "rules file")
	flags.BoolVarP(&options.verbose, "verbose", "v", false, "enable debug logging")
	flags.BoolVarP(&options.quiet, "quiet", "q", false, "only log errors")
	flags.StringVar(&options.logFormat, "log-format", "text", "log format: text or json")

	return command
}

func noArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}

	return apperr.Wrap(apperr.KindInput, fmt.Errorf("unexpected arguments: %v", args))
}

func logLevel(options rootOptions) slog.Level {
	if options.verbose {
		return slog.LevelDebug
	}
	if options.quiet {
		return slog.LevelError
	}

	return slog.LevelInfo
}
