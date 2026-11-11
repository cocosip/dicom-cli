package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileWritesNestedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "result.txt")
	if err := File(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(path); string(content) != "x" {
		t.Fatal("file output failed")
	}
}

func TestNewFileRejectsExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.txt")
	if err := NewFile(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := NewFile(path, []byte("y")); err == nil {
		t.Fatal("existing output accepted")
	}
}
