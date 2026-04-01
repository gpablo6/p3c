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
- Example: `gofmt -w main.go cmd/root.go internal/history/history.go`

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
- Run history package only: `go test ./internal/history`
- Run workflow package only: `go test ./internal/workflow`
- Run one test by exact name: `go test ./internal/history -run TestRewrite_MaxCommits_LimitsTraversal`
- Run one CLI test: `go test ./cmd -run TestMessageClean_GCAfterRunFlagRunsGC`
- Disable test caching when validating tricky changes: `go test -count=1 ./...`

## Single-Test Workflow

Prefer package-targeted test runs instead of `go test ./... -run ...`.

Good examples:

- `go test ./internal/history -run TestStripLine_RemovesTargetLine`
- `go test ./internal/history -run TestRewrite_MergeCommitParentsAreRewritten`
- `go test ./internal/history -run 'TestRewrite_(SingleCommitWithPattern|MaxCommits_LimitsTraversal)'`
- `go test ./cmd -run TestMessageClean_KeepBackupFlagRetainsBackupRef`

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

- `cmd/root_test.go`: command behavior for `message clean`, `identity rewrite`, and `backup clean`
- `cmd/message/clean_test.go`: command-local validation tests for message cleanup
- `cmd/backup/clean_test.go`: command-local backup cleanup output and behavior tests
- `cmd/identity/rewrite_test.go`: identity flag parsing and validation
- `internal/workflow/rewrite_test.go`: rewrite lifecycle and backup-ref behavior
- `internal/workflow/backup_test.go`: backup service listing and cleanup behavior
- `internal/history/history_test.go`: `StripLine`, history rewrite behavior, identity rewrite behavior, signature handling, merge handling, dry-run behavior

## Useful Existing Test Names

Examples already in the repo:

- `TestStripLine_RemovesTargetLine`
- `TestStripLine_DoesNotMatchTrailingWhitespaceVariant`
- `TestRewrite_MaxCommits_LimitsTraversal`
- `TestRewrite_MergeCommitParentsAreRewritten`
- `TestMessageClean_RemovesBackupByDefault`
- `TestMessageClean_RestoresHeadWhenRewriteFails`
- `TestMessageClean_GCAfterRunFlagRunsGC`
- `TestMessageClean_ReportsTemporaryBackupReference`
- `TestBackupClean_RemovesOnlyP3CBackupRefs`
- `TestIdentityRewrite_RewritesBothAuthorAndCommitter`
- `TestRewrite_RewritesExactMatchIdentity`

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
