package main

import (
	"strings"
	"testing"
)

func TestExecuteTranscodeFormatsListsRuntimeCodecsAndExperimentalHTJ2K(t *testing.T) {
	runtime, stdout, _ := testRuntime()
	if code := Execute([]string{"transcode", "formats", "--json"}, runtime); code != 0 {
		t.Fatalf("transcode formats exit code = %d, want 0", code)
	}
	for _, want := range []string{"explicit-vr-little-endian", "1.2.840.10008.1.2.4.201", `"experimental":true`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("formats output does not contain %q:\n%s", want, stdout.String())
		}
	}
}
