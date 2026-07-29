package app

import (
	"errors"
	"github.com/cocosip/dicom-cli/internal/files"
	"testing"
)

func TestRunCountsAndFailFast(t *testing.T) {
	entries := []files.Entry{{Path: "ok"}, {Path: "skip", Skipped: true, Reason: "filter"}, {Path: "bad"}, {Path: "later"}}
	summary := Run(entries, true, func(path string) error {
		if path == "bad" {
			return errors.New("bad")
		}
		return nil
	})
	if summary.Scanned != 4 || summary.Processed != 1 || summary.Skipped != 1 || summary.Failed != 1 {
		t.Fatalf("summary=%#v", summary)
	}
}

func TestRunContinuesAfterProcessingFailure(t *testing.T) {
	entries := []files.Entry{{Path: "bad"}, {Path: "later"}}
	processed := []string{}
	summary := Run(entries, false, func(path string) error {
		processed = append(processed, path)
		if path == "bad" {
			return errors.New("bad")
		}
		return nil
	})
	if summary.Processed != 1 || summary.Failed != 1 {
		t.Fatalf("summary=%#v", summary)
	}
	if len(processed) != 2 || processed[1] != "later" {
		t.Fatalf("processed=%#v", processed)
	}
}
