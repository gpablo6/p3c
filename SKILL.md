# Skill: p3c – Git History Co-Author Cleaner

## Overview

`p3c` (Pesky Claude Code Co-Author) is a CLI tool that rewrites a git
repository's commit history to remove a specific line from every commit
message.  It was designed to strip the AI co-author trailer:

```
Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
```

but it supports any literal line via the `--pattern` flag.

---

## When to use this skill

Use `p3c` when you need to:

- Remove an AI co-author trailer from every commit in a branch's history.
- Strip any exact-match line from commit messages across the full git log.
- Preview what would be removed before applying changes (dry-run).
- Automate history sanitisation as part of a CI/CD pipeline or release process.

---

## Prerequisites

- The repository must be a valid git repository (a `.git` directory must be
  reachable from the working directory).
- The user must have write access to the repository (the branch reference is
  updated in place).
- Go ≥ 1.21 is required **only** to build from source; the pre-built binary
  has no runtime dependencies.

---

## Installation

```bash
# From source
go install github.com/gpablo6/p3c@latest

# Or build locally
git clone https://github.com/gpablo6/p3c.git
cd p3c && go build -o p3c .
```

---

## Command reference

```
p3c [flags]

Flags:
  -p, --pattern string   Exact line to remove from commit messages
                         (default: "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>")
  -n, --dry-run          Preview changes without writing to the repo
  -v, --verbose          Print each commit SHA and subject as it is rewritten
  -h, --help             Show help
```

---

## Usage examples

### 1. Remove the default Claude trailer (standard use-case)

```bash
cd /path/to/repo
p3c
git push --force-with-lease
```

**Expected output:**
```
Scanned 47 commit(s). Cleaned 12 commit message(s).

Remember to force-push to update any remote branches:
  git push --force-with-lease
```

### 2. Dry-run (preview only, no writes)

```bash
p3c --dry-run
```

**Expected output:**
```
Dry-run mode – no changes will be written.
Scanned 47 commit(s). Would clean 12 commit message(s).
```

### 3. Remove a custom trailer

```bash
p3c --pattern "Signed-off-by: Automated Bot <bot@corp.example>"
```

### 4. Verbose output

```bash
p3c --verbose
# rewriting a1b2c3d: "feat: add payment flow"
# rewriting e4f5g6h: "fix: handle edge case"
# Scanned 47 commit(s). Cleaned 2 commit message(s).
```

---

## Behaviour details

| Scenario | Behaviour |
|---|---|
| Pattern found in commit message | Line is removed; consecutive blank lines collapsed; trailing blank lines stripped. |
| Pattern not found | Commit is left untouched; its SHA does not change. |
| Child commit of a rewritten parent | Rewritten to update parent pointer; message unchanged. |
| Multiple occurrences of pattern in one message | All occurrences removed. |
| Entire message is only the target line | Message becomes empty string. |
| No commits match | Repository unchanged; exit 0. |
| Dry-run | Reports count; writes nothing; exit 0. |
| Not inside a git repository | Error message; exit 1. |

---

## Caveats and safety notes

1. **History is rewritten.** All commit SHAs from the first affected commit to
   HEAD change.  Force-push is required to update remotes.
2. **Operate on a clean working tree** and ensure collaborators are not
   working on the branch.
3. **Take a backup** before running:
   ```bash
   git branch backup/$(git branch --show-current)
   ```
4. `p3c` modifies only the current HEAD branch.  Other branches, stashes, and
   tags that point to rewritten commits are **not** automatically updated.

---

## API (Go library)

`p3c` can also be used as a Go library:

```go
import (
    gogit "github.com/go-git/go-git/v5"
    "github.com/gpablo6/p3c/internal/cleaner"
)

repo, _ := gogit.PlainOpen("/path/to/repo")

result, err := cleaner.Clean(repo, cleaner.Config{
    Pattern: cleaner.DefaultPattern,  // or any literal line
    DryRun:  false,
    Verbose: true,
})

fmt.Printf("Scanned: %d, Modified: %d\n", result.CommitsScanned, result.CommitsModified)
```

### Key exported symbols

| Symbol | Type | Description |
|---|---|---|
| `DefaultPattern` | `const string` | The Claude Opus co-author trailer. |
| `Config` | `struct` | Configuration for a `Clean` call. |
| `Result` | `struct` | Summary of a completed `Clean` call. |
| `Clean(repo, cfg)` | `func` | Rewrites history in the given `go-git` repository. |
| `StripLine(msg, pat)` | `func` | Removes all occurrences of `pat` from a single message string. |

---

## Testing

```bash
go test ./...
```

The test suite covers:

- `StripLine`: line removal, no-match, multiple occurrences, consecutive blank
  line collapsing, empty message, message that is only the target line.
- `Clean`: no matching commits, single modified commit, multiple modified
  commits, parent-chain rewriting, metadata preservation, dry-run mode, custom
  pattern.
