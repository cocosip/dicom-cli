package main

import (
	"bytes"
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
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"ct-images", "summary", "basic", "ct-required-identifiers", "secondary-capture"} {
				if !bytes.Contains(content, []byte(want)) {
					t.Fatalf("generated %s does not contain starter rule %q: %s", test.name, want, content)
				}
			}
			runtime, _, stderr = testRuntime()
			if code := Execute([]string{"rules", "validate", path}, runtime); code != 0 {
				t.Fatalf("rules validate exit code = %d, stderr = %s", code, stderr.String())
			}
		})
	}
}
