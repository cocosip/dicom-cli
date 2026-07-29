// Package files provides bounded input discovery and safe output naming.
package files

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Entry struct {
	Path    string
	Skipped bool
	Reason  string
}

func DefaultOutputDirectory(workingDirectory, command string) string {
	return filepath.Join(workingDirectory, command)
}

func Scan(root string, recursive bool, include func(string) (bool, string, error)) ([]Entry, error) {
	entries := []Entry{}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !rootInfo.IsDir() {
		ok, reason, err := include(root)
		if err != nil {
			return nil, err
		}
		return []Entry{{Path: root, Skipped: !ok, Reason: reason}}, nil
	}
	visit := func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if item.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		if item.IsDir() {
			if item.Type()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			if !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !recursive && filepath.Dir(path) != root {
			return nil
		}
		ok, reason, err := include(path)
		if err != nil {
			return err
		}
		entries = append(entries, Entry{Path: path, Skipped: !ok, Reason: reason})
		return nil
	}
	if err := filepath.WalkDir(root, visit); err != nil {
		return nil, err
	}
	return entries, nil
}

func OutputPath(input, root, output string, preserve bool) (string, error) {
	absInput, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	if absInput == absOutput {
		return "", fmt.Errorf("output path is the input path")
	}
	name := filepath.Base(input)
	if preserve {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(absRoot, absInput)
		if err != nil {
			return "", err
		}
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return "", fmt.Errorf("input path is outside the input root")
		}
		name = relative
	}
	candidate := filepath.Join(absOutput, name)
	for index := 1; ; index++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
		extension := filepath.Ext(name)
		base := name[:len(name)-len(extension)]
		candidate = filepath.Join(absOutput, fmt.Sprintf("%s-%d%s", base, index, extension))
	}
}
