# Testing and Build Guide

This repository is a standard Go CLI project with no custom build system.

## Tooling Summary

- Go version from `go.mod`: `1.24.13`
- Build and test with the standard Go toolchain
- Git behavior is implemented in-process with `go-git`
- No `Makefile`, `Taskfile`, or `golangci-lint` config is present

## Build Commands

- Build the main package: `go build .`
- Build all packages: `go build ./...`
- Build the binary with the project name: `go build -o p3c .`
- Install from the current checkout: `go install .`

Use `go build ./...` after package wiring changes, exported API changes, or CLI integration changes.

## Formatting

- Format changed files with `gofmt -w`
- Example: `gofmt -w main.go cmd/root.go internal/cleaner/cleaner.go`

Run `gofmt` before tests when editing Go files.

## Linting

There is no dedicated linter configured in the repository.

For validation, use:

- `gofmt -w ...` for formatting
- `go build ./...` for compile checks
- `go test ./...` for behavior checks

Do not assume `golangci-lint`, `staticcheck`, or `go vet` are part of the required workflow unless the repo adds them later.

## Test Commands

- Run all tests: `go test ./...`
- Run one package: `go test ./cmd`
- Run cleaner package only: `go test ./internal/cleaner`
- Run one test by exact name: `go test ./internal/cleaner -run TestClean_CustomPattern`
- Run one CLI test: `go test ./cmd -run TestRun_GCAfterRunFlagRunsGC`
- Disable test caching when validating tricky changes: `go test -count=1 ./...`

## Single-Test Workflow

Prefer package-targeted test runs instead of `go test ./... -run ...`.

Good examples:

- `go test ./internal/cleaner -run TestStripLine_RemovesTargetLine`
- `go test ./internal/cleaner -run TestClean_MergeCommitParentsAreRewritten`
- `go test ./internal/cleaner -run 'TestClean_(CustomPattern|MaxCommits_LimitsTraversal)'`
- `go test ./cmd -run TestRun_KeepBackupFlagRetainsBackupRef`

Why this matters:

- Faster feedback
- Less noisy output
- Easier attribution when a failure happens

Use `go test ./... -run ...` only when you genuinely do not know which package owns the test.

## Suggested Validation Order

### Small internal logic change

1. `gofmt -w <changed-go-files>`
2. Run the most specific package test or single test
3. If behavior changed broadly, run `go test ./...`

### CLI or cross-package behavior change

1. `gofmt -w <changed-go-files>`
2. Run targeted tests in affected packages
3. Run `go test ./...`
4. Run `go build ./...`

### Documentation-only change

Usually no Go test run is required, but keep examples and flags aligned with the code.

## Current Test Layout

- `cmd/root_test.go`: command behavior, backup refs, rollback, GC hooks, backup naming, and backup messaging
- `internal/cleaner/cleaner_test.go`: `StripLine`, history rewrite behavior, metadata preservation, merge handling, dry-run behavior

## Useful Existing Test Names

Examples already in the repo:

- `TestStripLine_RemovesTargetLine`
- `TestStripLine_DoesNotMatchTrailingWhitespaceVariant`
- `TestClean_CustomPattern`
- `TestClean_MaxCommits_LimitsTraversal`
- `TestClean_MergeCommitParentsAreRewritten`
- `TestRun_RemovesBackupByDefault`
- `TestRun_RestoresHeadWhenCleanerFails`
- `TestRun_GCAfterRunFlagRunsGC`
- `TestRun_ReportsTemporaryBackupReference`
- `TestCreateBackupRef_AvoidsNameCollisions`

## When To Update Tests

Update or add tests when you change:

- Flag behavior or help text assumptions used by tests
- History rewrite behavior
- Backup reference behavior
- Dry-run behavior
- GC behavior
- User-visible output that existing assertions depend on

## README Sync

If you change commands, flags, behavior, or output wording, update `README.md` in the same change.
