# p3c — Pesky Claude Code Co-Author

`p3c` is a small, single-binary CLI tool written in Go that rewrites git
history safely for two focused use cases:

- removing exact-match lines from commit messages
- rewriting exact-match author/committer identities

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
For every rewritten commit reachable from `HEAD`:

1. The commit message is scanned for the target line.
2. Author and committer identities can optionally be rewritten when they match
   an exact source identity.
3. If a commit changes, a new commit object is created with only the requested
   fields updated.
4. All downstream commits that referenced a rewritten parent are also rewritten
   so the DAG remains internally consistent.
5. Any rewritten commit has its stale `PGPSignature` cleared because the old
   signature no longer matches the new commit object.
6. The current branch reference is updated to the new tip.
7. Before rewriting, p3c creates a backup branch reference and removes it after
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
p3c message clean --dry-run

# 2. Apply the commit-message clean:
p3c message clean

# 3. Force-push to update the remote branch:
git push --force-with-lease

# 4. Rewrite commit identities on the current branch:
p3c identity rewrite \
  --from "copilot-swe-agent[bot] <198982749+Copilot@users.noreply.github.com>" \
  --to "gpablo6 <gpablo6@outlook.com>"
```

---

## Usage

```
Usage:
  p3c [command]

Commands:
  backup      Manage p3c backup refs
  message     Rewrite commit messages
  identity    Rewrite author and committer identities in commit history
```

### Examples

```bash
# Remove the default Claude Opus co-author trailer:
p3c message clean

# Preview without writing (dry-run):
p3c message clean --dry-run

# Remove a custom trailer:
p3c message clean --pattern "Signed-off-by: Bot <bot@ci.example.com>"

# Verbose output (shows each rewritten commit SHA and subject):
p3c message clean --verbose

# Process only the latest 500 commits from HEAD:
p3c message clean --max-commits 500

# Keep backup reference after a successful run:
p3c message clean --keep-backup

# Remove local p3c backup refs after you've force-pushed:
p3c backup clean

# Prune unreachable loose objects after successful rewrite:
p3c message clean --gc-after-run

# Prune unreachable loose objects only if rewrite fails:
p3c message clean --gc-on-failure

# Rewrite both author and committer when they exactly match a source identity:
p3c identity rewrite \
  --from "copilot-swe-agent[bot] <198982749+Copilot@users.noreply.github.com>" \
  --to "gpablo6 <gpablo6@outlook.com>"

# Same rewrite using split flags:
p3c identity rewrite \
  --from-name "copilot-swe-agent[bot]" \
  --from-email "198982749+Copilot@users.noreply.github.com" \
  --to-name "gpablo6" \
  --to-email "gpablo6@outlook.com"

# Rewrite only the author field:
p3c identity rewrite --from "Old Name <old@example.com>" --to "New Name <new@example.com>" --scope author
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
- Identity rewrite semantics: identity rewrites require exact match on both
  name and email.
- Message preservation: aside from removing the matched line(s), commit
  messages are preserved as-is (including blank lines/newline style).
- Signature handling: rewritten commits have `PGPSignature` cleared because the
  original signature no longer applies to the rewritten commit object.
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
│   ├── root.go              # Root command and service wiring
│   ├── root_test.go         # Command-level integration tests
│   ├── message/
│   │   └── clean.go         # `p3c message clean`
│   ├── identity/
│   │   ├── rewrite.go       # `p3c identity rewrite`
│   │   └── rewrite_test.go  # Identity flag parsing tests
│   └── backup/
│       └── clean.go         # `p3c backup clean`
├── internal/
│   ├── workflow/
│   │   ├── rewrite.go       # Rewrite lifecycle orchestration
│   │   └── backup.go        # Backup ref listing and cleanup
│   └── history/
│       ├── history.go       # Core history-rewriting logic
│       └── history_test.go  # Unit test suite
├── SKILL.md                 # Agentic tool skill descriptor
└── docs/agents/             # Repo-specific style and testing guidance
```

---

## License

[MIT](LICENSE)
