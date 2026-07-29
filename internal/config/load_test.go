package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocateUsesExplicitCurrentThenUserWithoutMerging(t *testing.T) {
	root := t.TempDir()
	currentDir := filepath.Join(root, "current")
	userDir := filepath.Join(root, "user")
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}

	currentPath := filepath.Join(currentDir, DefaultFileName)
	userPath := filepath.Join(userDir, DefaultFileName)
	explicitPath := filepath.Join(root, "explicit.json")
	writeConfigFile(t, currentPath, "version: v1\ntargets: {}\n")
	writeConfigFile(t, userPath, "version: v1\ntargets: {}\n")
	writeConfigFile(t, explicitPath, "{\"version\":\"v1\",\"targets\":{}}")

	tests := []struct {
		name          string
		path          string
		envPath       string
		removeCurrent bool
		wantPath      string
		wantSource    Source
	}{
		{name: "explicit flag", path: explicitPath, wantPath: explicitPath, wantSource: SourceExplicit},
		{name: "environment path", envPath: explicitPath, wantPath: explicitPath, wantSource: SourceEnvironment},
		{name: "current directory", wantPath: currentPath, wantSource: SourceCurrent},
		{name: "user directory fallback", removeCurrent: true, wantPath: userPath, wantSource: SourceUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.removeCurrent {
				if err := os.Remove(currentPath); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { writeConfigFile(t, currentPath, "version: v1\ntargets: {}\n") })
			}

			location, err := Locate(LocateOptions{
				Path:          tt.path,
				WorkingDir:    currentDir,
				UserConfigDir: userDir,
				LookupEnv: envLookup(map[string]string{
					EnvConfig: tt.envPath,
				}),
			})
			if err != nil {
				t.Fatalf("Locate() error = %v", err)
			}
			if location.Path != tt.wantPath || location.Source != tt.wantSource {
				t.Fatalf("Locate() = %#v, want path %q and source %q", location, tt.wantPath, tt.wantSource)
			}
		})
	}
}

func TestLocateUsesDefaultsAndRejectsMissingExplicitPath(t *testing.T) {
	root := t.TempDir()
	options := LocateOptions{WorkingDir: root, UserConfigDir: filepath.Join(root, "user"), LookupEnv: envLookup(nil)}

	location, err := Locate(options)
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if location.Source != SourceDefaults || location.Path != "" {
		t.Fatalf("Locate() = %#v, want default location", location)
	}

	_, err = Locate(LocateOptions{Path: filepath.Join(root, "missing.yaml"), WorkingDir: root, UserConfigDir: root, LookupEnv: envLookup(nil)})
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Locate(missing explicit) error = %v, want not-exist error", err)
	}
}

func TestLoadChoosesOneFileRatherThanMergingCurrentAndUser(t *testing.T) {
	root := t.TempDir()
	currentDir := filepath.Join(root, "current")
	userDir := filepath.Join(root, "user")
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfigFile(t, filepath.Join(currentDir, DefaultFileName), "version: v1\ntargets:\n  current:\n    host: current.example.test\n    port: 104\n    calling_ae: DICOMCLI\n    called_ae: CURRENT\n")
	writeConfigFile(t, filepath.Join(userDir, DefaultFileName), "version: v1\ntargets:\n  user:\n    host: user.example.test\n    port: 105\n    calling_ae: DICOMCLI\n    called_ae: USER\n")

	config, location, err := Load(LocateOptions{WorkingDir: currentDir, UserConfigDir: userDir, LookupEnv: envLookup(nil)})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if location.Source != SourceCurrent {
		t.Fatalf("Load() source = %q, want current", location.Source)
	}
	if len(config.Targets) != 1 || config.Targets["current"].Host != "current.example.test" {
		t.Fatalf("Load() targets = %#v, want only current target", config.Targets)
	}
}

func TestResolveTargetUsesCommandEnvironmentConfigAndDefaultsInOrder(t *testing.T) {
	config := DefaultConfig()
	config.Targets["archive"] = PACSTarget{
		Host:           "config.example.test",
		Port:           11112,
		CallingAETitle: "CONFIGCALL",
		CalledAETitle:  "CONFIGCALLED",
		Timeouts:       Timeouts{Connect: 2 * time.Second},
	}

	commandTarget := "archive"
	commandHost := "command.example.test"
	commandPort := 104
	commandCallingAE := "COMMANDCALL"
	commandCalledAE := "COMMANDCALLED"

	target, err := ResolveTarget(config, TargetOverrides{
		Target:         &commandTarget,
		Host:           &commandHost,
		Port:           &commandPort,
		CallingAETitle: &commandCallingAE,
		CalledAETitle:  &commandCalledAE,
	}, envLookup(map[string]string{
		EnvTarget:    "missing",
		EnvHost:      "env.example.test",
		EnvPort:      "105",
		EnvCallingAE: "ENVCALL",
		EnvCalledAE:  "ENVCALLED",
	}))
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if target.Host != commandHost || target.Port != commandPort || target.CallingAETitle != commandCallingAE || target.CalledAETitle != commandCalledAE {
		t.Fatalf("ResolveTarget(command) = %#v, want command values", target)
	}
	if target.Timeouts.Connect != 2*time.Second || target.Timeouts.Associate != 30*time.Second || target.Timeouts.Idle != 5*time.Minute {
		t.Fatalf("ResolveTarget() timeouts = %#v, want target override with defaults", target.Timeouts)
	}

	target, err = ResolveTarget(config, TargetOverrides{}, envLookup(map[string]string{
		EnvTarget:    "archive",
		EnvHost:      "env.example.test",
		EnvPort:      "105",
		EnvCallingAE: "ENVCALL",
		EnvCalledAE:  "ENVCALLED",
	}))
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if target.Host != "env.example.test" || target.Port != 105 || target.CallingAETitle != "ENVCALL" || target.CalledAETitle != "ENVCALLED" {
		t.Fatalf("ResolveTarget(environment) = %#v, want environment values", target)
	}
}

func TestResolveTargetRejectsInvalidEnvironmentPort(t *testing.T) {
	_, err := ResolveTarget(DefaultConfig(), TargetOverrides{}, envLookup(map[string]string{EnvPort: "not-a-port"}))
	if err == nil {
		t.Fatal("ResolveTarget() error = nil, want invalid port error")
	}
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func envLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
