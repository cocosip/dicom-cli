package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/cocosip/dicom-cli/internal/apperr"
	"github.com/cocosip/dicom-cli/internal/config"
	"github.com/cocosip/dicom-cli/internal/i18n"
	"github.com/cocosip/dicom-cli/internal/logging"
)

// Runtime provides process dependencies to commands without requiring globals.
type Runtime struct {
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	Getwd         func() (string, error)
	UserConfigDir func() (string, error)
	LookupEnv     func(string) (string, bool)
}

// ProductionRuntime returns the dependencies used by the executable.
func ProductionRuntime() Runtime {
	return Runtime{
		Stdin:         os.Stdin,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Getwd:         os.Getwd,
		UserConfigDir: os.UserConfigDir,
		LookupEnv:     os.LookupEnv,
	}
}

type rootOptions struct {
	configPath string
	rulesPath  string
	verbose    bool
	quiet      bool
	logFormat  string
	localizer  i18n.Localizer
}

// Execute runs the root command and returns the process exit code.
func Execute(args []string, runtime Runtime) int {
	localizer := localizerForArgs(args, runtime)
	command := NewRootCommand(runtime, localizer)
	command.SetArgs(args)

	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintln(runtime.Stderr, localizer.ReplaceDiagnostic(err.Error()))
		return apperr.ExitCode(err)
	}

	return 0
}

// NewRootCommand builds the root Cobra command from injected process dependencies.
func NewRootCommand(runtime Runtime, localizer i18n.Localizer) *cobra.Command {
	options := rootOptions{logFormat: "text", localizer: localizer}

	command := &cobra.Command{
		Use:           "dicom-cli",
		Short:         localizer.Text(i18n.RootShort),
		Long:          localizer.Text(i18n.RootLong),
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
	command.SetUsageTemplate(localizedUsageTemplate(localizer))
	localizedHelpFlag(command, localizer)
	command.SetFlagErrorFunc(func(*cobra.Command, error) error {
		return apperr.Wrap(apperr.KindInput, fmt.Errorf("invalid command arguments"))
	})

	flags := command.PersistentFlags()
	flags.StringVarP(&options.configPath, "config", "c", "", localizer.Text(i18n.FlagConfig))
	flags.StringVarP(&options.rulesPath, "rules", "R", "", localizer.Text(i18n.FlagRules))
	flags.BoolVarP(&options.verbose, "verbose", "v", false, localizer.Text(i18n.FlagVerbose))
	flags.BoolVarP(&options.quiet, "quiet", "q", false, localizer.Text(i18n.FlagQuiet))
	flags.StringVar(&options.logFormat, "log-format", "text", localizer.Text(i18n.FlagLogFormat))
	command.AddCommand(newConfigCommand(runtime, &options))
	command.AddCommand(newRulesCommand(runtime, &options))
	command.AddCommand(newInspectCommand(runtime, &options, localizer))
	command.AddCommand(newValidateCommand(runtime, &options))
	command.AddCommand(newAnonymizeCommand(runtime, &options))
	command.AddCommand(newEditCommand(runtime, &options))
	command.AddCommand(newConvertCommand(runtime, &options))
	command.AddCommand(newEncapsulateCommand(runtime, &options))
	command.AddCommand(newTranscodeCommand(runtime, &options))
	command.AddCommand(newEchoCommand(runtime, &options))
	command.AddCommand(newSendCommand(runtime, &options))
	localizeCommandTree(command, localizer)

	return command
}

func localizerForArgs(args []string, runtime Runtime) i18n.Localizer {
	options, err := loadOptions(runtime, configuredConfigPath(args), nil)
	if err != nil {
		return i18n.New(i18n.English)
	}
	loaded, _, err := config.Load(options)
	if err != nil {
		return i18n.New(i18n.English)
	}
	return i18n.New(loaded.Language)
}

func configuredConfigPath(args []string) string {
	for index, argument := range args {
		switch {
		case argument == "--config" || argument == "-c":
			if index+1 < len(args) {
				return args[index+1]
			}
		case len(argument) > len("--config=") && argument[:len("--config=")] == "--config=":
			return argument[len("--config="):]
		case len(argument) > 2 && argument[:2] == "-c":
			return argument[2:]
		}
	}
	for index := 0; index+2 < len(args); index++ {
		if args[index] == "config" && args[index+1] == "validate" && !strings.HasPrefix(args[index+2], "-") {
			return args[index+2]
		}
	}
	return ""
}

func localizedUsageTemplate(localizer i18n.Localizer) string {
	return fmt.Sprintf(`%s{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

%s
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

%s
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

%s{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%s
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%s
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

%s{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

%s{{end}}
`,
		localizer.Text(i18n.HelpUsage),
		localizer.Text(i18n.HelpAliases),
		localizer.Text(i18n.HelpExamples),
		localizer.Text(i18n.HelpAvailableCommands),
		localizer.Text(i18n.HelpFlags),
		localizer.Text(i18n.HelpGlobalFlags),
		localizer.Text(i18n.HelpAdditionalTopics),
		localizer.Text(i18n.HelpMoreInformation, "{{.CommandPath}}"),
	)
}

func localizedHelpFlag(command *cobra.Command, localizer i18n.Localizer) {
	if command.Flags().Lookup("help") != nil {
		return
	}
	command.Flags().BoolP("help", "h", false, localizer.Text(i18n.FlagHelp))
}

func localizeCommandTree(root *cobra.Command, localizer i18n.Localizer) {
	if !localizer.IsChineseSimplified() {
		return
	}
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		localizedHelpFlag(command, localizer)
		command.Short = localizer.CommandShort(command.CommandPath(), command.Short)
		command.Long = localizer.CommandLong(command.CommandPath(), command.Long)
		command.Flags().VisitAll(func(flag *pflag.Flag) {
			flag.Usage = localizer.FlagUsage(flag.Name, flag.Usage)
		})
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(root)
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
