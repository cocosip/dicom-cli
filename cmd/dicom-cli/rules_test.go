package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRulesInitGeneratesAndValidatesYAMLAndJSON(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct{ name, format string }{{"yaml", "yaml"}, {"json", "json"}} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, "rules."+test.format)
			runtime, _, stderr := testRuntime()
			if code := Execute([]string{"rules", "init", path, "--format", test.format}, runtime); code != 0 {
				t.Fatalf("rules init exit code = %d, stderr = %s", code, stderr.String())
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal(err)
			}
			runtime, _, stderr = testRuntime()
			if code := Execute([]string{"rules", "validate", path}, runtime); code != 0 {
				t.Fatalf("rules validate exit code = %d, stderr = %s", code, stderr.String())
			}
		})
	}
}
