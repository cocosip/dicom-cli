package main

import (
	"path/filepath"
	"testing"
)

func TestRepositoryExamplesValidate(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		args []string
	}{
		{
			name: "configuration",
			args: []string{"config", "validate", filepath.Join(repositoryRoot, "examples", "dicom-cli.yaml")},
		},
		{
			name: "rules",
			args: []string{"rules", "validate", filepath.Join(repositoryRoot, "examples", "dicom-cli-rules.yaml")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, _, stderr := testRuntime()
			if code := Execute(test.args, runtime); code != 0 {
				t.Fatalf("%v exit code = %d, stderr = %s", test.args, code, stderr.String())
			}
		})
	}
}
