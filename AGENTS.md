# Repository Guidelines

## Project Structure & Module Organization

`dicom-cli` is a Go 1.26 Cobra CLI for DICOM file operations and DIMSE SCU work. The executable entry point and command wiring live in `cmd/dicom-cli/`; keep command parsing and user-facing output there. Put reusable behavior in focused `internal/` packages such as `dicom`, `validate`, `convert`, and `app`. Synthetic fixtures belong in `internal/testutil`; end-to-end command coverage is in `tests/integration`. Documentation is in `README.md` and `docs/`; starter YAML files are under `examples/`.

## DICOM Boundaries

Before adding DICOM or DIMSE behavior, inspect and reuse `github.com/cocosip/go-dicom`. Add narrow adapters for genuine gaps; do not create parallel protocol stacks.

## Build, Test, and Development Commands

Run these from the repository root:

```powershell
go build -o dicom-cli.exe ./cmd/dicom-cli # build the Windows CLI
go test ./...                             # run unit and integration tests
go vet ./...                              # run static checks used by CI
gofmt -w path\to\changed.go              # format edited Go files
```

On non-Windows hosts, omit `.exe`. CI runs the same formatting, vet, test, and `go build ./cmd/dicom-cli` checks on Windows, Linux, and macOS. Keep temporary build or package caches under the ignored `.cache/` directory.

## Coding Style & Naming Conventions

Use standard Go formatting with tabs; do not manually align code that `gofmt` will change. Keep package names lowercase and short. Follow existing file names such as `validate_command.go`, `validate_test.go`, and `echo_send.go`. Commands should retain Cobra's `Short`, `Long`, and `Example` help content. Preserve the CLI contract: command and flag names, JSON fields, exit codes, and DICOM identifiers are stable unless a change is explicitly intended.

## Testing Guidelines

Add focused tests beside the package under test and name them `TestBehaviorDescription`. Use table-driven subtests for shared setup. Command tests should construct an injected `Runtime` rather than depend on the real console, user directory, or environment. Use synthetic fixtures only; do not hard-code patient data, external DICOM paths, PACS addresses, or AE Titles. Run `go test ./...` before opening a pull request.

## Commit & Pull Request Guidelines

Recent history uses Conventional Commits with Gitmoji, for example `✨ feat(validate): add optional character set mismatch checks` or `👷 ci(release): build release archives`. Use an imperative summary and a scope when one module clearly owns the change. Pull requests should explain the behavior change, link the relevant issue, note documentation or configuration updates, and list the validation commands run. Include sample CLI output when it clarifies a changed user-facing contract.

## Cache Directory
- The `.cache` directory is used for project build and package caching, and is already added to `.gitignore`. During testing and debugging, relevant caches can be configured under this directory. No additional cache directories should be created.
