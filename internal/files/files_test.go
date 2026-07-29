package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAndOutputPath(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "a.dcm")
	if err := os.WriteFile(input, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "b.dcm"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := Scan(root, false, func(string) (bool, string, error) { return true, "", nil })
	if err != nil || len(entries) != 1 {
		t.Fatalf("Scan = %#v, %v", entries, err)
	}
	if _, err := OutputPath(input, root, input, false); err == nil {
		t.Fatal("same input/output accepted")
	}
	out := filepath.Join(root, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := OutputPath(input, root, out, false)
	if err != nil || filepath.Base(path) != "a.dcm" {
		t.Fatalf("OutputPath = %q, %v", path, err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err = OutputPath(input, root, out, false)
	if err != nil || filepath.Base(path) != "a-1.dcm" {
		t.Fatalf("OutputPath collision = %q, %v", path, err)
	}
}

func TestScanSingleFileAppliesFilter(t *testing.T) {
	input := filepath.Join(t.TempDir(), "single.dcm")
	if err := os.WriteFile(input, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := Scan(input, false, func(path string) (bool, string, error) {
		if path != input {
			t.Fatalf("filter path = %q, want %q", path, input)
		}
		return false, "filter", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].Skipped || entries[0].Reason != "filter" {
		t.Fatalf("Scan(single file) = %#v", entries)
	}
}

func TestScanRecursesWithoutFollowingDirectoryLinks(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	linked := filepath.Join(root, "linked")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "inside.dcm"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(child, linked); err != nil {
		t.Skipf("directory links are unavailable: %v", err)
	}
	entries, err := Scan(root, true, func(string) (bool, string, error) { return true, "", nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || filepath.Base(entries[0].Path) != "inside.dcm" {
		t.Fatalf("Scan(recursive) = %#v", entries)
	}
}

func TestDefaultOutputDirectoryUsesCommandSubdirectory(t *testing.T) {
	workingDirectory := t.TempDir()
	if got, want := DefaultOutputDirectory(workingDirectory, "anonymize"), filepath.Join(workingDirectory, "anonymize"); got != want {
		t.Fatalf("DefaultOutputDirectory() = %q, want %q", got, want)
	}
}

func TestOutputPathRejectsPreservedPathOutsideInputRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "input")
	output := filepath.Join(parent, "output")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.dcm")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OutputPath(outside, root, output, true); err == nil {
		t.Fatal("preserved path outside input root accepted")
	}
}
