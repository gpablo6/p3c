# p3c — Pesky Claude Code Co-Author

`p3c` is a small, single-binary CLI tool written in Go that scrubs AI
co-author trailers from every commit in a git repository's history.

It was built specifically to remove the line

```
Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
```

but it accepts any exact-match pattern via the `--pattern` flag, so you can
use it to clean any commit-message trailer you need to remove.

---

## How it works

`p3c` uses [go-git](https://github.com/go-git/go-git) – a pure-Go git
implementation – to rewrite history without spawning any external processes.
For every commit reachable from `HEAD`:

1. The commit message is scanned for the target line.
2. If found, a new commit object is created with only that exact line removed.
   All other commit-message content is preserved as-is.
3. All downstream commits that referenced a rewritten parent are also rewritten
   so the DAG remains internally consistent.
4. The current branch reference is updated to the new tip.
5. Before rewriting, p3c creates a backup branch reference and removes it after
   success unless you opt to keep it.

No files in the working tree are touched.

---

## Installation

### From source (requires Go 1.24.13+, matching `go.mod`)

```bash
go install github.com/gpablo6/p3c@latest
```

### Build locally

```bash
git clone https://github.com/gpablo6/p3c.git
cd p3c
go build -o p3c .
# Optionally move to a directory on $PATH:
mv p3c /usr/local/bin/
```

---

## Quick start

```bash
# Run inside any git repository on the branch you want to clean:
cd /path/to/your-repo

# 1. Preview changes without modifying the repo:
p3c --dry-run

# 2. Apply the clean:
p3c

# 3. Force-push to update the remote branch:
git push --force-with-lease
```

---

## Usage

```
Usage:
  p3c [flags]

Flags:
  -n, --dry-run          Show what would be changed without modifying the repository
      --gc-after-run     Prune unreachable loose objects after a successful run
      --gc-on-failure    Prune unreachable loose objects if cleaning fails after creating rewritten objects
  -h, --help             help for p3c
      --keep-backup      Keep the temporary backup branch after a successful rewrite
      --max-commits int  Limit traversal to the most recent N commits (default: all)
  -p, --pattern string   Exact line to remove from commit messages
                         (default "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>")
  -v, --verbose          Print each commit that is rewritten
```

### Examples

```bash
# Remove the default Claude Opus co-author trailer:
p3c

# Preview without writing (dry-run):
p3c --dry-run

# Remove a custom trailer:
p3c --pattern "Signed-off-by: Bot <bot@ci.example.com>"

# Verbose output (shows each rewritten commit SHA and subject):
p3c --verbose

# Process only the latest 500 commits from HEAD:
p3c --max-commits 500

# Keep backup reference after a successful run:
p3c --keep-backup

# Prune unreachable loose objects after successful rewrite:
p3c --gc-after-run

# Prune unreachable loose objects only if rewrite fails:
p3c --gc-on-failure
```

---

## ⚠️  Warning: destructive history rewrite

`p3c` rewrites git history. This changes commit SHAs. Before running on a
shared branch:

- Make sure no one else is working on the branch.
- `p3c` creates a temporary backup ref automatically. Use `--keep-backup` if
  you want to retain it after a successful run.
- If you want a manual long-lived backup branch, create one explicitly:
  `git branch backup/<branch-name>`.
- After running, force-push: `git push --force-with-lease`.

---

## Design notes

- Exact match semantics: p3c removes only lines that exactly match the target
  pattern. It intentionally does not normalize whitespace.
- Message preservation: aside from removing the matched line(s), commit
  messages are preserved as-is (including blank lines/newline style).
- Atomic ref update: rewritten objects are created first; the branch ref is
  updated at the end with compare-and-set semantics.
- Safety fallback: before rewrite, p3c creates a temporary backup reference.
  On failure it restores the original branch tip.
- Optional cleanup: `--gc-on-failure` / `--gc-after-run` use go-git prune to
  remove unreachable loose objects without spawning shell commands.

---

## Development

### Prerequisites

- Go 1.24.13+, matching `go.mod`

### Running tests

```bash
go test ./...
```

### Building

```bash
go build -o p3c .
```

### Project layout

```
p3c/
├── main.go                  # Entry point
├── cmd/
│   ├── root.go              # Cobra CLI command definition
│   └── root_test.go         # Command-level tests
├── internal/
│   └── cleaner/
│       ├── cleaner.go       # Core history-rewriting logic
│       └── cleaner_test.go  # Unit test suite
├── SKILL.md                 # Agentic tool skill descriptor
└── docs/agents/             # Repo-specific style and testing guidance
```

---

## License

[MIT](LICENSE)
