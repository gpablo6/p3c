// Package cleaner provides functionality to remove specific lines from git
// commit messages throughout a repository's history.
//
// It rewrites commits in place using go-git, preserving all commit metadata
// (author, committer, timestamps, trees) while only modifying the message.
// Downstream commits whose parent SHA changed as a result are also rewritten
// so the DAG remains consistent.
package cleaner

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Config holds the parameters for a Clean operation.
type Config struct {
	// Pattern is the exact line to remove from every commit message.
	// Defaults to the Claude co-author trailer when empty.
	Pattern string

	// DryRun reports what would be changed without actually modifying the
	// repository.
	DryRun bool

	// Verbose enables additional progress output to stdout.
	Verbose bool
}

// Result summarises the outcome of a Clean call.
type Result struct {
	// CommitsScanned is the total number of commits visited.
	CommitsScanned int

	// CommitsModified is the number of commits whose message was changed.
	CommitsModified int
}

// DefaultPattern is the co-author trailer inserted by Claude that this tool
// was specifically built to remove.
const DefaultPattern = "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"

// StripLine removes every line in msg that exactly equals pattern (after
// trimming trailing whitespace from each line).  Consecutive blank lines
// produced by the removal are collapsed into a single blank line, and any
// trailing blank lines are stripped from the result.  The original line
// endings are preserved for lines that are kept.
func StripLine(msg, pattern string) string {
	if msg == "" {
		return ""
	}

	lines := strings.Split(msg, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimRight(line, " \t") == pattern {
			continue
		}
		out = append(out, line)
	}

	// If nothing was removed, return unchanged to allow callers to detect a
	// no-op cheaply (string identity comparison).
	if len(out) == len(lines) {
		return msg
	}

	// Collapse consecutive blank lines.
	collapsed := make([]string, 0, len(out))
	prevBlank := false
	for _, line := range out {
		isBlank := strings.TrimSpace(line) == ""
		if isBlank && prevBlank {
			continue
		}
		collapsed = append(collapsed, line)
		prevBlank = isBlank
	}

	// Remove trailing blank lines (keep at most one trailing newline).
	end := len(collapsed)
	for end > 0 && strings.TrimSpace(collapsed[end-1]) == "" {
		end--
	}
	collapsed = collapsed[:end]

	if len(collapsed) == 0 {
		return ""
	}

	return strings.Join(collapsed, "\n") + "\n"
}

// Clean rewrites the commit history on the current HEAD branch of repo,
// removing every occurrence of cfg.Pattern from commit messages.
//
// The function walks the entire reachable history from HEAD, builds a mapping
// of old-hash → new-hash for any commit that needs rewriting (either because
// its own message changed, or because a parent was rewritten), and finally
// updates the HEAD branch reference to point at the new tip.
//
// When cfg.DryRun is true the function reports what it would do but does not
// write anything to the object store or update any references.
func Clean(repo *git.Repository, cfg Config) (*Result, error) {
	if cfg.Pattern == "" {
		cfg.Pattern = DefaultPattern
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD: %w", err)
	}

	// Collect commits from HEAD to root in newest-first order.
	iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("reading commit log: %w", err)
	}

	var commits []*object.Commit
	if err := iter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	// Reverse so we process from root → tip (parents before children).
	reverse(commits)

	result := &Result{CommitsScanned: len(commits)}

	// hashMap maps an original commit hash to its (possibly new) hash after
	// rewriting.  If a commit is not rewritten its entry maps to itself.
	hashMap := make(map[plumbing.Hash]plumbing.Hash, len(commits))

	for _, c := range commits {
		newMsg := StripLine(c.Message, cfg.Pattern)
		msgChanged := newMsg != c.Message

		// Rebuild parent list using the hashMap so we always reference the
		// rewritten parents.
		newParents := make([]plumbing.Hash, len(c.ParentHashes))
		parentsChanged := false
		for i, ph := range c.ParentHashes {
			if mapped, ok := hashMap[ph]; ok {
				newParents[i] = mapped
				if mapped != ph {
					parentsChanged = true
				}
			} else {
				newParents[i] = ph
			}
		}

		if !msgChanged && !parentsChanged {
			// This commit is untouched.
			hashMap[c.Hash] = c.Hash
			continue
		}

		// Only count commits where the message itself was modified, not those
		// rewritten purely because a parent hash changed.
		if msgChanged {
			result.CommitsModified++
			if cfg.Verbose {
				fmt.Printf("rewriting %s: %q\n", c.Hash, firstLine(c.Message))
			}
		}

		if cfg.DryRun {
			// In dry-run mode we can't write a new object, so we record the
			// original hash as unchanged.  This means subsequent commits that
			// depend on this one may not be counted as modified due to parent
			// changes – but the modified count for message changes is still
			// accurate, which is the most useful information for dry-run.
			hashMap[c.Hash] = c.Hash
			continue
		}

		newCommit := &object.Commit{
			Author:       c.Author,
			Committer:    c.Committer,
			Message:      newMsg,
			TreeHash:     c.TreeHash,
			ParentHashes: newParents,
		}

		obj := repo.Storer.NewEncodedObject()
		if err := newCommit.Encode(obj); err != nil {
			return result, fmt.Errorf("encoding rewritten commit %s: %w", c.Hash, err)
		}

		newHash, err := repo.Storer.SetEncodedObject(obj)
		if err != nil {
			return result, fmt.Errorf("storing rewritten commit %s: %w", c.Hash, err)
		}

		hashMap[c.Hash] = newHash
	}

	if cfg.DryRun {
		return result, nil
	}

	// Update the branch reference (the one HEAD points to) to the new tip.
	oldTip := ref.Hash()
	newTip, ok := hashMap[oldTip]
	if !ok || newTip == oldTip {
		// Nothing changed at the tip; no reference update needed.
		return result, nil
	}

	newRef := plumbing.NewHashReference(ref.Name(), newTip)
	if err := repo.Storer.SetReference(newRef); err != nil {
		return result, fmt.Errorf("updating branch reference: %w", err)
	}

	return result, nil
}

// reverse reverses a slice of *object.Commit in place.
func reverse(s []*object.Commit) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// firstLine returns the first line of s, used for verbose logging.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
