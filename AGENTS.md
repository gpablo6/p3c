# AGENTS.md

Agent guidance for `github.com/gpablo6/p3c`.

## Project

- Language: Go
- App type: single-binary CLI
- Module: `github.com/gpablo6/p3c`
- Entry point: `main.go`
- CLI package: `cmd/`
- Rewrite logic: `internal/cleaner/`

## Instruction Files Checked

- `AGENTS.md`: this file
- `.cursorrules`: not present
- `.cursor/rules/`: not present
- `.github/copilot-instructions.md`: not present
- `SKILL.md`: present; use as additional repo context

If Cursor or Copilot rule files are added later, treat them as repository-specific instructions and update this file to reference them.

## Read These Docs

- `docs/agents/testing.md` for build, format, test, and single-test commands
- `docs/agents/style.md` for code style, naming, error handling, and package-specific guidance

## Working Rules

- Keep edits small and focused
- Preserve safety messaging around destructive history rewrites
- Update tests with behavior changes
- Keep `README.md` in sync with user-facing command or output changes
- Do not assume custom lint tooling exists beyond standard Go tools

## Repo Landmarks

- `cmd/root.go`: Cobra command, flags, backup flow, CLI output
- `cmd/root_test.go`: command-level tests
- `internal/cleaner/cleaner.go`: history rewrite implementation
- `internal/cleaner/cleaner_test.go`: rewrite and message-strip tests
