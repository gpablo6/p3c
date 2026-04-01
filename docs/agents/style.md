# Code Style Guide

This codebase follows standard Go conventions with a few repo-specific expectations.

## Core Principles

- Prefer small, focused changes
- Preserve public behavior unless the task explicitly changes it
- Keep destructive rewrite behavior explicit and auditable
- Favor clarity over cleverness

## Formatting

- Always use `gofmt`
- Prefer readable control flow over compressed one-liners
- Let default Go formatting determine whitespace and alignment

## Imports

- Use normal Go import blocks
- Group imports as:
  - standard library
  - third-party packages
  - internal module imports
- Separate groups with one blank line
- Avoid aliases unless they solve a real naming conflict

## Naming

- Exported identifiers use `CamelCase`
- Unexported identifiers use `camelCase`
- Test names follow `TestXxx_Scenario`
- In `cmd/root.go`, flag state uses `flagX` globals
- Prefer repo domain terms such as `clean`, `pattern`, `backup`, `rewrite`, `repo`, and `commit`

## Types and API Shape

- Add doc comments for exported constants, types, and functions
- Keep structs focused and fields explicit
- Preserve the meaning of existing fields when extending `Config` or `Result`
- Avoid introducing ambiguous booleans when a clearer name or type would help

## Functions

- Prefer functions with one clear responsibility
- Return early on validation and operational failures
- Keep helpers small and descriptive
- Use package-level injectable vars only when tests need deterministic hooks

## Error Handling

- Wrap errors with context using `fmt.Errorf("...: %w", err)`
- Keep error text lowercase unless starting with a proper noun or flag name
- Mention the failed operation in the message
- Use warnings only for non-fatal cleanup issues
- Reserve `panic` for true programmer errors or impossible setup failures

Examples from the existing code style:

- `getting working directory: %w`
- `opening git repository: %w`
- `updating branch reference: %w`

## Comments

- Keep package comments where they add real context
- Comment non-obvious safety, iterator, and rewrite behavior
- Do not comment trivial control flow or obvious assignments
- Match the repo's current style: concise and practical

## CLI Output

- Use `cmd.Printf` and `cmd.Println` for command output in `cmd/`
- Keep user-facing text direct and stable
- If output text changes, update tests and `README.md` in the same change

## Safety Expectations

- Do not weaken safety messaging around force-pushes or backup refs
- Preserve rollback behavior when rewrites fail
- Preserve compare-and-set semantics around branch ref updates
- Keep dry-run side-effect free

## `internal/cleaner` Expectations

When touching `internal/cleaner`:

- Preserve commit metadata when rewriting commits
- Preserve exact message content except for the removed target line
- Do not normalize whitespace beyond the exact intended removal
- Maintain correct parent rewriting, including merge commits
- Keep traversal and rewrite behavior deterministic

## `cmd` Expectations

When touching `cmd/`:

- Keep help text and runtime behavior aligned with `README.md`
- Preserve explicit user messaging for destructive operations
- Keep hidden or reserved flags clearly marked
- Treat backup ref handling and GC orchestration as part of the command contract

## Testing Style

- Tests use `testing` with `testify/assert` and `testify/require`
- Use `require` for setup and fatal preconditions
- Use `assert` for value checks after setup passes
- Mark helpers with `t.Helper()`
- Keep tests deterministic
- Reset injected globals during cleanup when tests modify them

## Before Finishing A Change

- Run `gofmt -w` on changed Go files
- Run the most targeted relevant tests first
- Run `go test ./...` for broad behavior changes
- Run `go build ./...` when package wiring or integration changes
- Update docs when user-facing behavior changes
