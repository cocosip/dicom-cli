package app

import "github.com/cocosip/dicom-cli/internal/files"

type Summary struct {
	Scanned, Processed, Skipped, Failed int
	Skips                               []files.Entry
}

func Run(entries []files.Entry, failFast bool, process func(string) error) Summary {
	summary := Summary{Scanned: len(entries)}
	for _, entry := range entries {
		if entry.Skipped {
			summary.Skipped++
			summary.Skips = append(summary.Skips, entry)
			continue
		}
		if err := process(entry.Path); err != nil {
			summary.Failed++
			if failFast {
				return summary
			}
			continue
		}
		summary.Processed++
	}
	return summary
}
