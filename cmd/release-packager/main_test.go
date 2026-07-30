package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestCreateArchiveIncludesReleaseFiles(t *testing.T) {
	t.Parallel()

	for _, format := range []archiveFormat{archiveZIP, archiveTarGz} {
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "dicom-cli_0.0.0_test")
			for _, name := range []string{
				"dicom-cli",
				"README.md",
				"docs/usage.md",
				"examples/dicom-cli.yaml",
				"examples/dicom-cli-rules.yaml",
			} {
				path := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(name), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			archivePath := filepath.Join(t.TempDir(), "release"+format.extension())
			if err := createArchive(root, archivePath, format); err != nil {
				t.Fatalf("createArchive() error = %v", err)
			}

			got, err := archiveEntries(archivePath, format)
			if err != nil {
				t.Fatalf("archiveEntries() error = %v", err)
			}
			want := []string{
				"dicom-cli_0.0.0_test/README.md",
				"dicom-cli_0.0.0_test/dicom-cli",
				"dicom-cli_0.0.0_test/docs/usage.md",
				"dicom-cli_0.0.0_test/examples/dicom-cli-rules.yaml",
				"dicom-cli_0.0.0_test/examples/dicom-cli.yaml",
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("archive entries = %v, want %v", got, want)
			}
		})
	}
}

func TestNewBuildCommandDisablesVCSStamping(t *testing.T) {
	command := newBuildCommand("C:/source", "C:/output/dicom-cli.exe", target{goos: "windows", goarch: "amd64"})
	if !slices.Contains(command.Args, "-buildvcs=false") {
		t.Fatalf("build arguments = %v, want -buildvcs=false", command.Args)
	}
}
