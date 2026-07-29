package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

const (
	DefaultFileName = "dicom-cli.yaml"

	EnvConfig    = "DICOM_CLI_CONFIG"
	EnvTarget    = "DICOM_CLI_TARGET"
	EnvHost      = "DICOM_CLI_HOST"
	EnvPort      = "DICOM_CLI_PORT"
	EnvCallingAE = "DICOM_CLI_CALLING_AE"
	EnvCalledAE  = "DICOM_CLI_CALLED_AE"
)

// Source identifies where a configuration was selected from.
type Source string

const (
	SourceExplicit    Source = "explicit"
	SourceEnvironment Source = "environment"
	SourceCurrent     Source = "current"
	SourceUser        Source = "user"
	SourceDefaults    Source = "defaults"
)

// Location identifies the one configuration file selected by discovery.
type Location struct {
	Path   string
	Source Source
}

// LocateOptions supplies the process-dependent values required for discovery.
type LocateOptions struct {
	Path          string
	WorkingDir    string
	UserConfigDir string
	LookupEnv     func(string) (string, bool)
}

// TargetOverrides represents explicitly supplied command-line target values.
// A nil pointer means the command did not supply that field.
type TargetOverrides struct {
	Target         *string
	Host           *string
	Port           *int
	CallingAETitle *string
	CalledAETitle  *string
}

// Locate selects one configuration file without combining values from files.
func Locate(options LocateOptions) (Location, error) {
	if options.Path != "" {
		return explicitLocation(options.Path, SourceExplicit)
	}
	if path, ok := lookup(options.LookupEnv, EnvConfig); ok && path != "" {
		return explicitLocation(path, SourceEnvironment)
	}

	if location, found, err := implicitLocation(filepath.Join(options.WorkingDir, DefaultFileName), SourceCurrent); err != nil {
		return Location{}, err
	} else if found {
		return location, nil
	}
	if location, found, err := implicitLocation(filepath.Join(options.UserConfigDir, DefaultFileName), SourceUser); err != nil {
		return Location{}, err
	} else if found {
		return location, nil
	}

	return Location{Source: SourceDefaults}, nil
}

// Load reads and validates the one file selected by Locate, or returns defaults.
func Load(options LocateOptions) (Config, Location, error) {
	location, err := Locate(options)
	if err != nil {
		return Config{}, Location{}, err
	}

	config := DefaultConfig()
	if location.Path == "" {
		return config, location, nil
	}

	loader := viper.New()
	loader.SetConfigFile(location.Path)
	if err := loader.ReadInConfig(); err != nil {
		return Config{}, Location{}, fmt.Errorf("read configuration %q: %w", location.Path, err)
	}
	if err := loader.UnmarshalExact(&config, viper.DecodeHook(mapstructure.StringToTimeDurationHookFunc())); err != nil {
		return Config{}, Location{}, fmt.Errorf("decode configuration %q: %w", location.Path, err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, Location{}, fmt.Errorf("validate configuration %q: %w", location.Path, err)
	}

	return config, location, nil
}

// ResolveTarget applies command-line and environment values over configured
// target values. Timeout values inherit from the global configuration defaults.
func ResolveTarget(config Config, command TargetOverrides, lookupEnv func(string) (string, bool)) (PACSTarget, error) {
	targetName := stringOverride(command.Target, lookupEnv, EnvTarget)
	target := PACSTarget{Timeouts: config.Timeouts}
	if targetName != "" {
		configured, ok := config.Targets[targetName]
		if !ok {
			return PACSTarget{}, fmt.Errorf("target %q is not configured", targetName)
		}
		target = configured
		target.Timeouts = mergeTimeouts(config.Timeouts, configured.Timeouts)
	}

	if value, ok := lookup(lookupEnv, EnvHost); ok && value != "" {
		target.Host = value
	}
	if value, ok := lookup(lookupEnv, EnvPort); ok && value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return PACSTarget{}, fmt.Errorf("%s must be an integer port: %w", EnvPort, err)
		}
		target.Port = port
	}
	if value, ok := lookup(lookupEnv, EnvCallingAE); ok && value != "" {
		target.CallingAETitle = value
	}
	if value, ok := lookup(lookupEnv, EnvCalledAE); ok && value != "" {
		target.CalledAETitle = value
	}

	if command.Host != nil {
		target.Host = *command.Host
	}
	if command.Port != nil {
		target.Port = *command.Port
	}
	if command.CallingAETitle != nil {
		target.CallingAETitle = *command.CallingAETitle
	}
	if command.CalledAETitle != nil {
		target.CalledAETitle = *command.CalledAETitle
	}

	if target.Port != 0 && (target.Port < 1 || target.Port > 65535) {
		return PACSTarget{}, fmt.Errorf("effective target port must be between 1 and 65535")
	}
	if target.CallingAETitle != "" && !validAETitle(target.CallingAETitle) {
		return PACSTarget{}, fmt.Errorf("effective calling AE Title must be 1 to 16 printable ASCII characters")
	}
	if target.CalledAETitle != "" && !validAETitle(target.CalledAETitle) {
		return PACSTarget{}, fmt.Errorf("effective called AE Title must be 1 to 16 printable ASCII characters")
	}

	return target, nil
}

func explicitLocation(path string, source Source) (Location, error) {
	if _, err := os.Stat(path); err != nil {
		return Location{}, err
	}
	return Location{Path: path, Source: source}, nil
}

func implicitLocation(path string, source Source) (Location, bool, error) {
	if _, err := os.Stat(path); err == nil {
		return Location{Path: path, Source: source}, true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return Location{}, false, nil
	} else {
		return Location{}, false, err
	}
}

func mergeTimeouts(parent, child Timeouts) Timeouts {
	if child.Connect > 0 {
		parent.Connect = child.Connect
	}
	if child.Associate > 0 {
		parent.Associate = child.Associate
	}
	if child.Idle > 0 {
		parent.Idle = child.Idle
	}
	return parent
}

func stringOverride(command *string, lookupEnv func(string) (string, bool), envKey string) string {
	if command != nil {
		return *command
	}
	if value, ok := lookup(lookupEnv, envKey); ok {
		return value
	}
	return ""
}

func lookup(lookupEnv func(string) (string, bool), key string) (string, bool) {
	if lookupEnv == nil {
		return "", false
	}
	return lookupEnv(key)
}
