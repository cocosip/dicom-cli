package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewJSONWritesToProvidedWriter(t *testing.T) {
	var output bytes.Buffer

	logger, err := New(slog.LevelInfo, "json", &output)
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("ready")

	if !strings.Contains(output.String(), `"msg":"ready"`) {
		t.Fatalf("output = %q, want JSON log entry", output.String())
	}
}

func TestNewHonorsLevel(t *testing.T) {
	var output bytes.Buffer

	logger, err := New(slog.LevelError, "text", &output)
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("hidden")
	logger.Error("visible")

	if strings.Contains(output.String(), "hidden") {
		t.Fatalf("output = %q, contains filtered info log", output.String())
	}
	if !strings.Contains(output.String(), "visible") {
		t.Fatalf("output = %q, does not contain error log", output.String())
	}
}
