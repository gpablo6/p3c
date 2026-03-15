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
2. If found, a new commit object is created with the line removed, and
   consecutive blank lines produced by the removal are collapsed.
3. All downstream commits that referenced a rewritten parent are also rewritten
   so the DAG remains internally consistent.
4. The current branch reference is updated to the new tip.

No files in the working tree are touched.

---

## Installation

### From source (requires Go ≥ 1.21)

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
  -h, --help             help for p3c
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
```

---

## ⚠️  Warning: destructive history rewrite

`p3c` rewrites git history. This changes commit SHAs. Before running on a
shared branch:

- Make sure no one else is working on the branch.
- Take a backup: `git branch backup/<branch-name>`.
- After running, force-push: `git push --force-with-lease`.

---

## Development

### Prerequisites

- Go ≥ 1.21

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
│   └── root.go              # Cobra CLI command definition
├── internal/
│   └── cleaner/
│       ├── cleaner.go       # Core history-rewriting logic
│       └── cleaner_test.go  # Unit test suite
└── SKILL.md                 # Agentic tool skill descriptor
```

---

## License

[MIT](LICENSE)
