// Package logging constructs application loggers.
package logging

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/cocosip/dicom-cli/internal/apperr"
)

// New constructs a logger that writes only to writer.
func New(level slog.Level, format string, writer io.Writer) (*slog.Logger, error) {
	options := &slog.HandlerOptions{Level: level}

	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(writer, options)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(writer, options)), nil
	default:
		return nil, apperr.Wrap(apperr.KindInput, fmt.Errorf("unsupported log format %q", format))
	}
}
