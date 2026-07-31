package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestExecuteHelpListsP0GlobalFlags(t *testing.T) {
	runtime, stdout, _ := testRuntime()

	if code := Execute([]string{"--help"}, runtime); code != 0 {
		t.Fatalf("Execute(--help) = %d, want 0", code)
	}

	for _, flag := range []string{"--config", "--rules", "--verbose", "--quiet", "--log-format"} {
		if !strings.Contains(stdout.String(), flag) {
			t.Fatalf("help output does not contain %q:\n%s", flag, stdout.String())
		}
	}
}

func TestExecuteHelpExplainsEveryProjectCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "root", args: []string{"--help"}, want: "configuration, rules, file processing, and DIMSE"},
		{name: "config", args: []string{"config", "--help"}, want: "Create, validate, and maintain"},
		{name: "config init", args: []string{"config", "init", "--help"}, want: "Existing files are never overwritten"},
		{name: "config validate", args: []string{"config", "validate", "--help"}, want: "built-in defaults are validated"},
		{name: "config target", args: []string{"config", "target", "--help"}, want: "selected configuration file"},
		{name: "config target list", args: []string{"config", "target", "list", "--help"}, want: "one name per line"},
		{name: "config target add", args: []string{"config", "target", "add", "--help"}, want: "requires all four connection fields"},
		{name: "config target update", args: []string{"config", "target", "update", "--help"}, want: "Only explicitly supplied fields"},
		{name: "config target remove", args: []string{"config", "target", "remove", "--help"}, want: "removes the target from the selected configuration file"},
		{name: "rules", args: []string{"rules", "--help"}, want: "Rule files provide named filters"},
		{name: "rules init", args: []string{"rules", "init", "--help"}, want: "Existing files are never overwritten"},
		{name: "rules validate", args: []string{"rules", "validate", "--help"}, want: "Unknown fields are rejected"},
		{name: "inspect", args: []string{"inspect", "--help"}, want: "never modifies the source file"},
		{name: "validate", args: []string{"validate", "--help"}, want: "all independent findings"},
		{name: "edit", args: []string{"edit", "--help"}, want: "At least one edit operation is required"},
		{name: "anonymize", args: []string{"anonymize", "--help"}, want: "UID mappings are shared across the batch"},
		{name: "convert", args: []string{"convert", "--help"}, want: "DICOM input is exported"},
		{name: "convert image", args: []string{"convert", "image", "--help"}, want: "Frame numbers start at 1"},
		{name: "convert json", args: []string{"convert", "json", "--help"}, want: "PixelData is summarized by default"},
		{name: "encapsulate", args: []string{"encapsulate", "--help"}, want: "External images are imported"},
		{name: "encapsulate image", args: []string{"encapsulate", "image", "--help"}, want: "Study and Series UIDs are shared"},
		{name: "transcode", args: []string{"transcode", "--help"}, want: "--to accepts a transfer syntax alias or standard UID"},
		{name: "transcode formats", args: []string{"transcode", "formats", "--help"}, want: "registered in this binary"},
		{name: "echo", args: []string{"echo", "--help"}, want: "does not modify remote data"},
		{name: "send", args: []string{"send", "--help"}, want: "does not transcode source instances"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, stdout, _ := testRuntime()
			if code := Execute(tt.args, runtime); code != 0 {
				t.Fatalf("Execute(%v) = %d, want 0", tt.args, code)
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("help does not contain %q:\n%s", tt.want, stdout.String())
			}
		})
	}
}

func TestExecuteAcceptsP0GlobalFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "config", args: []string{"-c", "custom.yaml"}},
		{name: "rules", args: []string{"-R", "rules.yaml"}},
		{name: "verbose", args: []string{"-v"}},
		{name: "quiet", args: []string{"-q"}},
		{name: "json logs", args: []string{"--log-format", "json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, _, _ := testRuntime()

			if code := Execute(tt.args, runtime); code != 0 {
				t.Fatalf("Execute(%v) = %d, want 0", tt.args, code)
			}
		})
	}
}

func TestExecuteReturnsInputCodeForInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unsupported log format", args: []string{"--log-format", "xml"}},
		{name: "verbose and quiet conflict", args: []string{"-v", "-q"}},
		{name: "unexpected positional argument", args: []string{"input.dcm"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, stdout, stderr := testRuntime()

			if code := Execute(tt.args, runtime); code != 2 {
				t.Fatalf("Execute(%v) = %d, want 2", tt.args, code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr is empty, want diagnostic")
			}
		})
	}
}

func TestExecuteUsesInjectedRuntime(t *testing.T) {
	cwdCalls := 0
	envCalls := 0
	runtime := Runtime{
		Stdin:  strings.NewReader(""),
		Stdout: io.Discard,
		Stderr: io.Discard,
		Getwd: func() (string, error) {
			cwdCalls++
			return ".", nil
		},
		UserConfigDir: func() (string, error) {
			return ".", nil
		},
		LookupEnv: func(string) (string, bool) {
			envCalls++
			return "", false
		},
	}

	if code := Execute([]string{"--help"}, runtime); code != 0 {
		t.Fatalf("Execute(--help) = %d, want 0", code)
	}
	if cwdCalls != 0 || envCalls != 0 {
		t.Fatalf("Getwd calls = %d, LookupEnv calls = %d, want 0", cwdCalls, envCalls)
	}
}

func testRuntime() (Runtime, *bytes.Buffer, *bytes.Buffer) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	return Runtime{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
		Getwd: func() (string, error) {
			return ".", nil
		},
		UserConfigDir: func() (string, error) {
			return ".", nil
		},
		LookupEnv: func(string) (string, bool) {
			return "", false
		},
	}, &stdout, &stderr
}
