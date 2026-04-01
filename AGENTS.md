# AGENTS.md

Agent guidance for `github.com/gpablo6/p3c`.

## Project

- Language: Go
- App type: single-binary CLI
- Module: `github.com/gpablo6/p3c`
- Entry point: `main.go`
- CLI package: `cmd/`
- Rewrite logic: `internal/history/`
- Workflow orchestration: `internal/workflow/`

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

- `cmd/root.go`: root command and subcommand registration
- `cmd/message/clean.go`: message cleanup command
- `cmd/identity/rewrite.go`: identity rewrite command
- `cmd/backup/clean.go`: backup cleanup command
- `cmd/root_test.go`: command-level tests
- `internal/workflow/`: rewrite lifecycle and backup ref orchestration
- `internal/history/history.go`: history rewrite implementation
- `internal/history/history_test.go`: rewrite and message-strip tests
