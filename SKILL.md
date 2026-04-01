---
name: p3c
description: Rewrite git commit history on the current branch to remove an exact-match line from commit messages, especially `Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>`. Use when sanitizing commit messages, removing AI co-author trailers, previewing a history rewrite, or operating the p3c CLI or cleaner library.
license: MIT
compatibility: Requires a git repository. Building from source requires Go 1.24.13+; running the compiled CLI does not.
metadata:
  owner: gpablo6
  language: go
---

# p3c

Use this skill when working on the `p3c` project itself or when operating the
compiled `p3c` binary against a repository.

## What it does

`p3c` rewrites the current branch history in two explicit ways:

- remove exact-match lines from commit messages
- rewrite exact-match author and committer identities

Default pattern:

```text
Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
```

The tool preserves all other commit-message content as-is. It does not collapse
blank lines or normalize whitespace.

## How to use it

Run from inside the repository whose current branch you want to rewrite.

Preview only:

```bash
p3c message clean --dry-run
```

Apply the rewrite:

```bash
p3c message clean
git push --force-with-lease
```

Rewrite identities:

```bash
p3c identity rewrite \
  --from "copilot-swe-agent[bot] <198982749+Copilot@users.noreply.github.com>" \
  --to "gpablo6 <gpablo6@outlook.com>"
```

Use a custom exact-match line:

```bash
p3c message clean --pattern "Signed-off-by: Automated Bot <bot@example.com>"
```

Limit traversal to recent history:

```bash
p3c message clean --max-commits 500
```

Keep the backup ref after a successful run:

```bash
p3c message clean --keep-backup
```

Remove local p3c backup refs:

```bash
p3c backup clean
```

Prune unreachable loose objects:

```bash
p3c message clean --gc-after-run
p3c message clean --gc-on-failure
```

## Current behavior

- Operates on the checked-out `HEAD` branch.
- Rewrites downstream commits when parent hashes change.
- Supports explicit identity rewrites as a separate command.
- Uses explicit subcommands instead of a destructive default root action.
- Creates a temporary backup branch ref before rewriting.
- Restores the original branch ref if cleaning fails.
- Removes the temporary backup ref on success unless `--keep-backup` is used.
- Optional prune flags use go-git native prune for unreachable loose objects.
- Rewritten commits have `PGPSignature` cleared because signatures become stale
  once the commit object changes.
- If history changes, rewritten SHAs must be force-pushed manually.

## Key files

- `cmd/root.go`: root command and subcommand registration.
- `cmd/message/clean.go`: message cleanup command.
- `cmd/identity/rewrite.go`: identity rewrite command.
- `cmd/backup/clean.go`: backup cleanup command.
- `internal/workflow/rewrite.go`: rewrite lifecycle orchestration.
- `internal/workflow/backup.go`: backup ref listing and cleanup.
- `internal/history/history.go`: exact line removal and commit graph rewrite.
- `internal/history/history_test.go`: core rewrite and message-preservation tests.
- `cmd/root_test.go`: command-level tests using temp filesystem repositories.

## Maintenance notes

- Keep `SKILL.md` aligned with actual CLI behavior and flags.
- If flags or rewrite semantics change, update this file and `README.md`
  together.
- Preserve the exact-match semantics in documentation: remove only the target
  line, leave all other message bytes intact.
