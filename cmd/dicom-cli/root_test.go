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
