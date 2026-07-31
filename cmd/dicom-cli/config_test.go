package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitGeneratesAndValidatesYAMLAndJSON(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name   string
		path   string
		format string
		prefix string
	}{
		{name: "yaml", path: filepath.Join(root, "dicom-cli.yaml"), format: "yaml", prefix: "version: v1"},
		{name: "json", path: filepath.Join(root, "dicom-cli.json"), format: "json", prefix: "{"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, _, stderr := testRuntime()
			if code := Execute([]string{"config", "init", tt.path, "--format", tt.format}, runtime); code != 0 {
				t.Fatalf("config init exit code = %d, stderr = %s", code, stderr.String())
			}

			content, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", tt.path, err)
			}
			if !strings.HasPrefix(strings.TrimSpace(string(content)), tt.prefix) {
				t.Fatalf("generated %s = %q, want prefix %q", tt.name, content, tt.prefix)
			}
			for _, want := range []string{"local-pacs", "pacs.example.test", "DICOMCLI", "PACS"} {
				if !bytes.Contains(content, []byte(want)) {
					t.Fatalf("generated %s does not contain starter target value %q: %s", tt.name, want, content)
				}
			}

			runtime, _, stderr = testRuntime()
			if code := Execute([]string{"config", "validate", tt.path}, runtime); code != 0 {
				t.Fatalf("config validate exit code = %d, stderr = %s", code, stderr.String())
			}

			runtime, _, stderr = testRuntime()
			if code := Execute([]string{"config", "init", tt.path, "--format", tt.format}, runtime); code != 2 {
				t.Fatalf("config init existing exit code = %d, want 2; stderr = %s", code, stderr.String())
			}
		})
	}
}

func TestConfigTargetCRUDUsesExplicitTemporaryConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dicom-cli.yaml")
	if err := os.WriteFile(path, []byte("version: v1\ntargets: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runtime, _, stderr := testRuntime()
	if code := Execute([]string{"--config", path, "config", "target", "add", "archive", "--host", "pacs.example.test", "--port", "104", "--calling-ae", "DICOMCLI", "--called-ae", "ARCHIVE"}, runtime); code != 0 {
		t.Fatalf("config target add exit code = %d, stderr = %s", code, stderr.String())
	}

	runtime, stdout, stderr := testRuntime()
	if code := Execute([]string{"--config", path, "config", "target", "list"}, runtime); code != 0 {
		t.Fatalf("config target list exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "archive") {
		t.Fatalf("config target list stdout = %q, want archive", stdout.String())
	}

	runtime, _, stderr = testRuntime()
	if code := Execute([]string{"--config", path, "config", "target", "update", "archive", "--host", "new-pacs.example.test"}, runtime); code != 0 {
		t.Fatalf("config target update exit code = %d, stderr = %s", code, stderr.String())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("new-pacs.example.test")) {
		t.Fatalf("updated configuration = %s, want new host", content)
	}

	runtime, _, stderr = testRuntime()
	if code := Execute([]string{"--config", path, "config", "target", "remove", "archive"}, runtime); code != 0 {
		t.Fatalf("config target remove exit code = %d, stderr = %s", code, stderr.String())
	}

	runtime, stdout, stderr = testRuntime()
	if code := Execute([]string{"--config", path, "config", "target", "list"}, runtime); code != 0 {
		t.Fatalf("config target list after remove exit code = %d, stderr = %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "archive") {
		t.Fatalf("config target list stdout = %q, do not want archive", stdout.String())
	}
}

func TestConfigLanguageCommandPersistsLanguageForLaterCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dicom-cli.yaml")
	if err := os.WriteFile(path, []byte("version: v1\nlanguage: en\ntargets: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runtime, stdout, stderr := testRuntime()
	if code := Execute([]string{"--config", path, "config", "language", "zh-CN"}, runtime); code != 0 {
		t.Fatalf("config language exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "language=zh-CN\n" {
		t.Fatalf("config language stdout = %q, want %q", stdout.String(), "language=zh-CN\n")
	}

	runtime, stdout, stderr = testRuntime()
	if code := Execute([]string{"--config", path, "inspect", "--help"}, runtime); code != 0 {
		t.Fatalf("inspect help exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "查看一个 DICOM 文件") {
		t.Fatalf("persisted Chinese help = %q", stdout.String())
	}
}

func TestRootLanguageCommandPersistsLanguage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dicom-cli.yaml")
	if err := os.WriteFile(path, []byte("version: v1\nlanguage: en\ntargets: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runtime, stdout, stderr := testRuntime()
	if code := Execute([]string{"--config", path, "lang", "zh-CN"}, runtime); code != 0 {
		t.Fatalf("lang exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "language=zh-CN\n" {
		t.Fatalf("lang stdout = %q, want %q", stdout.String(), "language=zh-CN\n")
	}

	runtime, stdout, stderr = testRuntime()
	if code := Execute([]string{"--config", path, "inspect", "--help"}, runtime); code != 0 {
		t.Fatalf("inspect help exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "查看一个 DICOM 文件") {
		t.Fatalf("persisted Chinese help = %q", stdout.String())
	}
}

func TestRootLanguageCommandCreatesUserConfigurationWhenNoneExists(t *testing.T) {
	workingDir := t.TempDir()
	userConfigDir := filepath.Join(t.TempDir(), "dicom-cli")
	runtime, stdout, stderr := testRuntime()
	runtime.Getwd = func() (string, error) {
		return workingDir, nil
	}
	runtime.UserConfigDir = func() (string, error) {
		return userConfigDir, nil
	}

	if code := Execute([]string{"lang", "zh-CN"}, runtime); code != 0 {
		t.Fatalf("lang exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "language=zh-CN\n" {
		t.Fatalf("lang stdout = %q, want %q", stdout.String(), "language=zh-CN\n")
	}

	userConfigPath := filepath.Join(userConfigDir, "dicom-cli.yaml")
	content, err := os.ReadFile(userConfigPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", userConfigPath, err)
	}
	if !strings.Contains(string(content), "language: zh-CN") {
		t.Fatalf("created user configuration = %q, want language setting", content)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "dicom-cli.yaml")); !os.IsNotExist(err) {
		t.Fatalf("working directory configuration existence error = %v, want not exist", err)
	}

	runtime, stdout, stderr = testRuntime()
	runtime.Getwd = func() (string, error) {
		return workingDir, nil
	}
	runtime.UserConfigDir = func() (string, error) {
		return userConfigDir, nil
	}
	if code := Execute([]string{"inspect", "--help"}, runtime); code != 0 {
		t.Fatalf("inspect help exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "查看一个 DICOM 文件") {
		t.Fatalf("persisted Chinese help = %q", stdout.String())
	}
}
