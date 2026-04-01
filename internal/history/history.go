// Package history rewrites git commit history while preserving the graph.
package history

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// Identity describes a git commit identity.
type Identity struct {
	Name  string
	Email string
}

// IdentityRewrite configures exact-match author/committer rewriting.
type IdentityRewrite struct {
	From             Identity
	To               Identity
	RewriteAuthor    bool
	RewriteCommitter bool
}

// Config holds the parameters for a Rewrite operation.
type Config struct {
	Pattern         string
	DryRun          bool
	Verbose         bool
	MaxCommits      int
	IdentityRewrite *IdentityRewrite
}

// Result summarizes the outcome of a Rewrite call.
type Result struct {
	CommitsScanned  int
	CommitsModified int
}

// DefaultPattern is the built-in commit-message line removed by `p3c message clean`.
const DefaultPattern = "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"

// StripLine removes exact-match lines from a commit message without changing
// any other whitespace or blank-line layout.
func StripLine(msg, pattern string) string {
	if msg == "" {
		return ""
	}
	parts := strings.SplitAfter(msg, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSuffix(part, "\n")
		if line == pattern {
			continue
		}
		out = append(out, part)
	}
	if len(out) == len(parts) {
		return msg
	}
	return strings.Join(out, "")
}

// Rewrite rewrites commits reachable from HEAD while preserving the commit DAG.
func Rewrite(repo *git.Repository, cfg Config) (*Result, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD: %w", err)
	}
	iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("reading commit log: %w", err)
	}

	var commits []*object.Commit
	if err := iter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		if cfg.MaxCommits > 0 && len(commits) >= cfg.MaxCommits {
			return storer.ErrStop
		}
		return nil
	}); err != nil && err != storer.ErrStop {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	reverse(commits)
	result := &Result{CommitsScanned: len(commits)}
	hashMap := make(map[plumbing.Hash]plumbing.Hash, len(commits))

	for _, c := range commits {
		newMsg := c.Message
		if cfg.Pattern != "" {
			newMsg = StripLine(c.Message, cfg.Pattern)
		}
		msgChanged := newMsg != c.Message
		newAuthor := c.Author
		newCommitter := c.Committer
		identityChanged := false
		if cfg.IdentityRewrite != nil {
			newAuthor, newCommitter, identityChanged = rewriteIdentity(c.Author, c.Committer, cfg.IdentityRewrite)
		}

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

		if !msgChanged && !identityChanged && !parentsChanged {
			hashMap[c.Hash] = c.Hash
			continue
		}
		if msgChanged || identityChanged {
			result.CommitsModified++
			if cfg.Verbose {
				fmt.Printf("rewriting %s: %q\n", c.Hash, firstLine(c.Message))
			}
		}
		if cfg.DryRun {
			hashMap[c.Hash] = c.Hash
			continue
		}

		newCommit := &object.Commit{
			Author:       newAuthor,
			Committer:    newCommitter,
			MergeTag:     c.MergeTag,
			PGPSignature: "",
			Message:      newMsg,
			TreeHash:     c.TreeHash,
			ParentHashes: newParents,
			Encoding:     c.Encoding,
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
	oldTip := ref.Hash()
	newTip, ok := hashMap[oldTip]
	if !ok || newTip == oldTip {
		return result, nil
	}
	newRef := plumbing.NewHashReference(ref.Name(), newTip)
	if err := repo.Storer.CheckAndSetReference(newRef, ref); err != nil {
		return result, fmt.Errorf("updating branch reference: %w", err)
	}
	return result, nil
}

func rewriteIdentity(author, committer object.Signature, cfg *IdentityRewrite) (object.Signature, object.Signature, bool) {
	changed := false
	if cfg.RewriteAuthor && matchesIdentity(author, cfg.From) {
		author.Name = cfg.To.Name
		author.Email = cfg.To.Email
		changed = true
	}
	if cfg.RewriteCommitter && matchesIdentity(committer, cfg.From) {
		committer.Name = cfg.To.Name
		committer.Email = cfg.To.Email
		changed = true
	}
	return author, committer, changed
}

func matchesIdentity(sig object.Signature, identity Identity) bool {
	return sig.Name == identity.Name && sig.Email == identity.Email
}

func reverse(s []*object.Commit) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
