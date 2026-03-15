package cleaner_test

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gpablo6/p3c/internal/cleaner"
)

const defaultPattern = "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"

// makeRepo creates an in-memory git repository with a given set of commits.
// Each element of messages is used as a commit message; commits are created
// in order (index 0 is the root).  The helper returns the repository and the
// filesystem-backed worktree so that callers can make further changes if
// needed.
func makeRepo(t *testing.T, messages []string) *git.Repository {
	t.Helper()

	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	require.NoError(t, err, "git init")

	sig := &object.Signature{
		Name:  "Test User",
		Email: "test@example.com",
		When:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// We need a tree for every commit.  Since we have no worktree we create
	// an empty tree object directly.
	emptyTreeHash, err := storeEmptyTree(repo)
	require.NoError(t, err, "store empty tree")

	var parentHash plumbing.Hash
	for i, msg := range messages {
		var parents []plumbing.Hash
		if i > 0 {
			parents = []plumbing.Hash{parentHash}
		}

		commit := &object.Commit{
			Author:       *sig,
			Committer:    *sig,
			Message:      msg,
			TreeHash:     emptyTreeHash,
			ParentHashes: parents,
		}

		obj := store.NewEncodedObject()
		require.NoError(t, commit.Encode(obj), "encode commit")

		hash, err := store.SetEncodedObject(obj)
		require.NoError(t, err, "store commit")

		parentHash = hash
	}

	// Point HEAD → main and main → last commit.
	ref := plumbing.NewHashReference("refs/heads/main", parentHash)
	require.NoError(t, store.SetReference(ref), "set branch ref")

	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
	require.NoError(t, store.SetReference(headRef), "set HEAD")

	return repo
}

// storeEmptyTree stores an empty git tree object and returns its hash.
func storeEmptyTree(repo *git.Repository) (plumbing.Hash, error) {
	tree := &object.Tree{}
	obj := repo.Storer.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return repo.Storer.SetEncodedObject(obj)
}

// commitMessages returns the list of commit messages on the HEAD branch of
// repo, ordered from newest to oldest.
func commitMessages(t *testing.T, repo *git.Repository) []string {
	t.Helper()

	ref, err := repo.Head()
	require.NoError(t, err)

	iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	require.NoError(t, err)

	var msgs []string
	require.NoError(t, iter.ForEach(func(c *object.Commit) error {
		msgs = append(msgs, c.Message)
		return nil
	}))
	return msgs
}

// ---------------------------------------------------------------------------
// StripLine tests
// ---------------------------------------------------------------------------

func TestStripLine_RemovesTargetLine(t *testing.T) {
	input := "feat: add feature\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n"
	want := "feat: add feature\n"
	got := cleaner.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_NoMatch_Unchanged(t *testing.T) {
	input := "feat: add feature\n\nCo-Authored-By: Someone Else <other@example.com>\n"
	got := cleaner.StripLine(input, defaultPattern)
	assert.Equal(t, input, got)
}

func TestStripLine_RemovesOnlyMatchingLine(t *testing.T) {
	input := "fix: bug\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\nCo-Authored-By: Human <human@example.com>\n"
	want := "fix: bug\n\nCo-Authored-By: Human <human@example.com>\n"
	got := cleaner.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_MultipleOccurrences(t *testing.T) {
	input := "fix: bug\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n"
	want := "fix: bug\n"
	got := cleaner.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_CollapsesConsecutiveBlankLines(t *testing.T) {
	input := "feat: thing\n\nSome details.\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n\nMore text.\n"
	want := "feat: thing\n\nSome details.\n\nMore text.\n"
	got := cleaner.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_EmptyMessage(t *testing.T) {
	got := cleaner.StripLine("", defaultPattern)
	assert.Equal(t, "", got)
}

func TestStripLine_OnlyTargetLine(t *testing.T) {
	input := "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n"
	got := cleaner.StripLine(input, defaultPattern)
	assert.Equal(t, "", got)
}

// ---------------------------------------------------------------------------
// Clean (full history rewrite) tests
// ---------------------------------------------------------------------------

func TestClean_NoMatchingCommits(t *testing.T) {
	messages := []string{
		"initial commit\n",
		"feat: add stuff\n",
	}
	repo := makeRepo(t, messages)
	originalTip, _ := repo.Head()

	cfg := cleaner.Config{
		Pattern: defaultPattern,
	}
	result, err := cleaner.Clean(repo, cfg)
	require.NoError(t, err)

	assert.Equal(t, 2, result.CommitsScanned)
	assert.Equal(t, 0, result.CommitsModified)

	// HEAD should be unchanged.
	newTip, _ := repo.Head()
	assert.Equal(t, originalTip.Hash(), newTip.Hash())
}

func TestClean_SingleCommitWithPattern(t *testing.T) {
	messages := []string{
		"initial commit\n",
		"feat: add stuff\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n",
	}
	repo := makeRepo(t, messages)

	cfg := cleaner.Config{
		Pattern: defaultPattern,
	}
	result, err := cleaner.Clean(repo, cfg)
	require.NoError(t, err)

	assert.Equal(t, 2, result.CommitsScanned)
	assert.Equal(t, 1, result.CommitsModified)

	// Check the resulting messages (newest first).
	msgs := commitMessages(t, repo)
	assert.Equal(t, "feat: add stuff\n", msgs[0])
	assert.Equal(t, "initial commit\n", msgs[1])
}

func TestClean_MultipleCommitsWithPattern(t *testing.T) {
	messages := []string{
		"initial commit\n",
		"feat: first\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n",
		"feat: second\n",
		"feat: third\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n",
	}
	repo := makeRepo(t, messages)

	cfg := cleaner.Config{
		Pattern: defaultPattern,
	}
	result, err := cleaner.Clean(repo, cfg)
	require.NoError(t, err)

	assert.Equal(t, 4, result.CommitsScanned)
	assert.Equal(t, 2, result.CommitsModified)

	msgs := commitMessages(t, repo)
	assert.Equal(t, "feat: third\n", msgs[0])
	assert.Equal(t, "feat: second\n", msgs[1])
	assert.Equal(t, "feat: first\n", msgs[2])
	assert.Equal(t, "initial commit\n", msgs[3])
}

func TestClean_PreservesCommitMetadata(t *testing.T) {
	targetMsg := "feat: my feature\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n"
	repo := makeRepo(t, []string{"initial commit\n", targetMsg})

	ref, err := repo.Head()
	require.NoError(t, err)
	origCommit, err := repo.CommitObject(ref.Hash())
	require.NoError(t, err)

	cfg := cleaner.Config{Pattern: defaultPattern}
	_, err = cleaner.Clean(repo, cfg)
	require.NoError(t, err)

	ref, err = repo.Head()
	require.NoError(t, err)
	newCommit, err := repo.CommitObject(ref.Hash())
	require.NoError(t, err)

	// Author and committer must be preserved.
	assert.Equal(t, origCommit.Author.Name, newCommit.Author.Name)
	assert.Equal(t, origCommit.Author.Email, newCommit.Author.Email)
	assert.Equal(t, origCommit.Committer.Name, newCommit.Committer.Name)
	assert.Equal(t, origCommit.Committer.Email, newCommit.Committer.Email)
	// Tree must be preserved.
	assert.Equal(t, origCommit.TreeHash, newCommit.TreeHash)
}

func TestClean_DryRun_DoesNotModifyRepo(t *testing.T) {
	messages := []string{
		"initial commit\n",
		"feat: add stuff\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n",
	}
	repo := makeRepo(t, messages)
	originalTip, _ := repo.Head()

	cfg := cleaner.Config{
		Pattern: defaultPattern,
		DryRun:  true,
	}
	result, err := cleaner.Clean(repo, cfg)
	require.NoError(t, err)

	// Should still report what would be modified.
	assert.Equal(t, 1, result.CommitsModified)

	// But HEAD must be unchanged.
	newTip, _ := repo.Head()
	assert.Equal(t, originalTip.Hash(), newTip.Hash())
}

func TestClean_AllCommitsClean_HeadUnchanged(t *testing.T) {
	messages := []string{
		"chore: init\n",
		"docs: update readme\n",
		"fix: typo\n",
	}
	repo := makeRepo(t, messages)
	originalTip, _ := repo.Head()

	cfg := cleaner.Config{Pattern: defaultPattern}
	result, err := cleaner.Clean(repo, cfg)
	require.NoError(t, err)

	assert.Equal(t, 3, result.CommitsScanned)
	assert.Equal(t, 0, result.CommitsModified)

	newTip, _ := repo.Head()
	assert.Equal(t, originalTip.Hash(), newTip.Hash())
}

func TestClean_CustomPattern(t *testing.T) {
	pattern := "Signed-off-by: Bot <bot@ci.example.com>"
	messages := []string{
		"initial commit\n",
		"ci: run tests\n\nSigned-off-by: Bot <bot@ci.example.com>\n",
	}
	repo := makeRepo(t, messages)

	cfg := cleaner.Config{Pattern: pattern}
	result, err := cleaner.Clean(repo, cfg)
	require.NoError(t, err)

	assert.Equal(t, 1, result.CommitsModified)
	msgs := commitMessages(t, repo)
	assert.Equal(t, "ci: run tests\n", msgs[0])
}
