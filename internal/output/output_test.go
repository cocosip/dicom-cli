package output

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryRejectsMultipleResults(t *testing.T) {
	if err := Binary(&bytes.Buffer{}, [][]byte{{1}, {2}}); err == nil {
		t.Fatal("multiple output accepted")
	}
}

func TestTextJSONBinaryAndFileOutput(t *testing.T) {
	var buffer bytes.Buffer
	if err := Text(&buffer, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&buffer, map[string]string{"x": "y"}); err != nil {
		t.Fatal(err)
	}
	if err := Binary(&buffer, [][]byte{{1}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "nested", "result.txt")
	if err := File(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(path); string(content) != "x" {
		t.Fatal("file output failed")
	}
}
