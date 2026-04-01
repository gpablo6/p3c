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

`p3c` rewrites the current branch history and removes every commit-message line
that exactly matches a target pattern.

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
p3c --dry-run
```

Apply the rewrite:

```bash
p3c
git push --force-with-lease
```

Use a custom exact-match line:

```bash
p3c --pattern "Signed-off-by: Automated Bot <bot@example.com>"
```

Limit traversal to recent history:

```bash
p3c --max-commits 500
```

Keep the backup ref after a successful run:

```bash
p3c --keep-backup
```

Prune unreachable loose objects:

```bash
p3c --gc-after-run
p3c --gc-on-failure
```

## Current behavior

- Operates on the checked-out `HEAD` branch.
- Rewrites downstream commits when parent hashes change.
- Creates a temporary backup branch ref before rewriting.
- Restores the original branch ref if cleaning fails.
- Removes the temporary backup ref on success unless `--keep-backup` is used.
- Optional prune flags use go-git native prune for unreachable loose objects.
- If history changes, rewritten SHAs must be force-pushed manually.

## Key files

- `cmd/root.go`: CLI flags, backup lifecycle, rollback, optional prune flow.
- `internal/cleaner/cleaner.go`: exact line removal and commit graph rewrite.
- `internal/cleaner/cleaner_test.go`: core rewrite and message-preservation tests.
- `cmd/root_test.go`: command-level tests using temp filesystem repositories.

## Maintenance notes

- Keep `SKILL.md` aligned with actual CLI behavior and flags.
- If flags or rewrite semantics change, update this file and `README.md`
  together.
- Preserve the exact-match semantics in documentation: remove only the target
  line, leave all other message bytes intact.
