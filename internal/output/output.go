package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Text(w io.Writer, value string) error { _, err := fmt.Fprintln(w, value); return err }
func JSON(w io.Writer, value any) error    { return json.NewEncoder(w).Encode(value) }
func Binary(w io.Writer, values [][]byte) error {
	if len(values) != 1 {
		return fmt.Errorf("binary stdout requires exactly one result")
	}
	_, err := w.Write(values[0])
	return err
}

func File(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}
