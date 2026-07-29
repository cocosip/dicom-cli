package apperr

import (
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	base := errors.New("failed")

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "unclassified error is operational", err: base, want: 1},
		{name: "operational error", err: Wrap(KindOperation, base), want: 1},
		{name: "input error", err: Wrap(KindInput, base), want: 2},
		{name: "validation error", err: Wrap(KindValidation, base), want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
