package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type archiveFormat string

const (
	archiveZIP   archiveFormat = "zip"
	archiveTarGz archiveFormat = "tar.gz"
)

func (format archiveFormat) extension() string {
	if format == archiveZIP {
		return ".zip"
	}
	return ".tar.gz"
}

type target struct {
	goos, goarch string
	format       archiveFormat
}

var releaseTargets = []target{
	{goos: "windows", goarch: "amd64", format: archiveZIP},
	{goos: "linux", goarch: "amd64", format: archiveTarGz},
	{goos: "linux", goarch: "arm64", format: archiveTarGz},
	{goos: "darwin", goarch: "amd64", format: archiveTarGz},
	{goos: "darwin", goarch: "arm64", format: archiveTarGz},
}

var releaseFiles = []string{
	"README.md",
	"docs/usage.md",
	"examples/dicom-cli.yaml",
	"examples/dicom-cli-rules.yaml",
}

func main() {
	var version, output string
	flag.StringVar(&version, "version", "", "release version, with or without a leading v")
	flag.StringVar(&output, "output", "dist", "directory for release archives")
	flag.Parse()

	if err := packageRelease(version, output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func packageRelease(version, output string) error {
	version, err := normalizeVersion(version)
	if err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	staging, err := os.MkdirTemp(output, ".release-staging-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	for _, target := range releaseTargets {
		if err := packageTarget(root, staging, output, version, target); err != nil {
			return err
		}
	}
	return nil
}

func normalizeVersion(version string) (string, error) {
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		return "", errors.New("--version is required")
	}
	for _, character := range version {
		switch {
		case character >= '0' && character <= '9':
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character == '.' || character == '-':
		default:
			return "", fmt.Errorf("invalid release version %q", version)
		}
	}
	return version, nil
}

func packageTarget(root, staging, output, version string, target target) error {
	name := fmt.Sprintf("dicom-cli_%s_%s_%s", version, target.goos, target.goarch)
	archivePath := filepath.Join(output, name+target.format.extension())
	if _, err := os.Stat(archivePath); err == nil {
		return fmt.Errorf("release archive already exists: %s", archivePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect release archive: %w", err)
	}

	stagedRoot := filepath.Join(staging, name)
	if err := os.MkdirAll(stagedRoot, 0o755); err != nil {
		return fmt.Errorf("create staging root: %w", err)
	}
	binaryName := "dicom-cli"
	if target.goos == "windows" {
		binaryName += ".exe"
	}
	if err := buildBinary(root, filepath.Join(stagedRoot, binaryName), target); err != nil {
		return err
	}
	for _, source := range releaseFiles {
		if err := copyFile(filepath.Join(root, filepath.FromSlash(source)), filepath.Join(stagedRoot, filepath.FromSlash(source))); err != nil {
			return fmt.Errorf("stage %s: %w", source, err)
		}
	}
	if err := createArchive(stagedRoot, archivePath, target.format); err != nil {
		return fmt.Errorf("create %s: %w", archivePath, err)
	}
	if err := verifyReleaseArchive(archivePath, target.format, name, binaryName); err != nil {
		return err
	}
	return nil
}

func buildBinary(root, output string, target target) error {
	command := newBuildCommand(root, output, target)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("build %s/%s: %w", target.goos, target.goarch, err)
	}
	return nil
}

func newBuildCommand(root, output string, target target) *exec.Cmd {
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", output, "./cmd/dicom-cli")
	command.Dir = root
	command.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH="+target.goarch, "CGO_ENABLED=0")
	return command
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	info, err := input.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func createArchive(root, destination string, format archiveFormat) error {
	if format == archiveZIP {
		return createZIP(root, destination)
	}
	if format == archiveTarGz {
		return createTarGz(root, destination)
	}
	return fmt.Errorf("unsupported archive format %q", format)
}

func createZIP(root, destination string) error {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)
	err = walkReleaseFiles(root, func(path, name string, info fs.FileInfo) error {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		return copyArchiveFile(writer, path)
	})
	closeErr := archive.Close()
	fileCloseErr := output.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return fileCloseErr
}

func createTarGz(root, destination string) error {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(output)
	archive := tar.NewWriter(gzipWriter)
	err = walkReleaseFiles(root, func(path, name string, info fs.FileInfo) error {
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		return copyArchiveFile(archive, path)
	})
	archiveCloseErr := archive.Close()
	gzipCloseErr := gzipWriter.Close()
	fileCloseErr := output.Close()
	if err != nil {
		return err
	}
	if archiveCloseErr != nil {
		return archiveCloseErr
	}
	if gzipCloseErr != nil {
		return gzipCloseErr
	}
	return fileCloseErr
}

func walkReleaseFiles(root string, visit func(path, name string, info fs.FileInfo) error) error {
	parent := filepath.Dir(root)
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}
		return visit(path, filepath.ToSlash(name), info)
	})
}

func copyArchiveFile(output io.Writer, path string) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	_, err = io.Copy(output, input)
	return err
}

func archiveEntries(path string, format archiveFormat) ([]string, error) {
	if format == archiveZIP {
		return zipEntries(path)
	}
	if format == archiveTarGz {
		return tarGzEntries(path)
	}
	return nil, fmt.Errorf("unsupported archive format %q", format)
}

func zipEntries(path string) ([]string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = archive.Close() }()
	entries := make([]string, 0, len(archive.File))
	for _, file := range archive.File {
		if !file.FileInfo().IsDir() {
			entries = append(entries, file.Name)
		}
	}
	sort.Strings(entries)
	return entries, nil
}

func tarGzEntries(path string) ([]string, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = input.Close() }()
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gzipReader.Close() }()
	archive := tar.NewReader(gzipReader)
	var entries []string
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg {
			entries = append(entries, header.Name)
		}
	}
	sort.Strings(entries)
	return entries, nil
}

func verifyReleaseArchive(path string, format archiveFormat, root, binary string) error {
	entries, err := archiveEntries(path, format)
	if err != nil {
		return fmt.Errorf("inspect release archive %s: %w", path, err)
	}
	want := []string{
		root + "/" + binary,
		root + "/README.md",
		root + "/docs/usage.md",
		root + "/examples/dicom-cli.yaml",
		root + "/examples/dicom-cli-rules.yaml",
	}
	sort.Strings(want)
	if strings.Join(entries, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("release archive %s entries = %v, want %v", path, entries, want)
	}
	return nil
}
