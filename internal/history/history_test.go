package history_test

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gpablo6/p3c/internal/history"
)

const defaultPattern = "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"

func makeRepo(t *testing.T, messages []string) *git.Repository {
	t.Helper()
	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	require.NoError(t, err)

	sig := &object.Signature{Name: "Test User", Email: "test@example.com", When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	emptyTreeHash, err := storeEmptyTree(repo)
	require.NoError(t, err)

	var parentHash plumbing.Hash
	for i, msg := range messages {
		var parents []plumbing.Hash
		if i > 0 {
			parents = []plumbing.Hash{parentHash}
		}
		commit := &object.Commit{Author: *sig, Committer: *sig, Message: msg, TreeHash: emptyTreeHash, ParentHashes: parents}
		obj := store.NewEncodedObject()
		require.NoError(t, commit.Encode(obj))
		hash, err := store.SetEncodedObject(obj)
		require.NoError(t, err)
		parentHash = hash
	}

	ref := plumbing.NewHashReference("refs/heads/main", parentHash)
	require.NoError(t, store.SetReference(ref))
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
	require.NoError(t, store.SetReference(headRef))
	return repo
}

func storeEmptyTree(repo *git.Repository) (plumbing.Hash, error) {
	tree := &object.Tree{}
	obj := repo.Storer.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return repo.Storer.SetEncodedObject(obj)
}

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

func TestStripLine_RemovesTargetLine(t *testing.T) {
	input := "feat: add feature\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n"
	want := "feat: add feature\n\n"
	got := history.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_NoMatch_Unchanged(t *testing.T) {
	input := "feat: add feature\n\nCo-Authored-By: Someone Else <other@example.com>\n"
	got := history.StripLine(input, defaultPattern)
	assert.Equal(t, input, got)
}

func TestStripLine_RemovesOnlyMatchingLine(t *testing.T) {
	input := "fix: bug\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\nCo-Authored-By: Human <human@example.com>\n"
	want := "fix: bug\n\nCo-Authored-By: Human <human@example.com>\n"
	got := history.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_MultipleOccurrences(t *testing.T) {
	input := "fix: bug\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n"
	want := "fix: bug\n\n"
	got := history.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_PreservesSurroundingBlankLines(t *testing.T) {
	input := "feat: thing\n\nSome details.\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n\nMore text.\n"
	want := "feat: thing\n\nSome details.\n\n\nMore text.\n"
	got := history.StripLine(input, defaultPattern)
	assert.Equal(t, want, got)
}

func TestStripLine_EmptyMessage(t *testing.T) {
	got := history.StripLine("", defaultPattern)
	assert.Equal(t, "", got)
}

func TestStripLine_OnlyTargetLine(t *testing.T) {
	input := "Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>\n"
	got := history.StripLine(input, defaultPattern)
	assert.Equal(t, "", got)
}

func TestStripLine_DoesNotMatchTrailingWhitespaceVariant(t *testing.T) {
	input := "feat: add feature\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>  \n"
	got := history.StripLine(input, defaultPattern)
	assert.Equal(t, input, got)
}

func TestRewrite_NoMatchingCommits(t *testing.T) {
	repo := makeRepo(t, []string{"initial commit\n", "feat: add stuff\n"})
	originalTip, _ := repo.Head()
	result, err := history.Rewrite(repo, history.Config{Pattern: defaultPattern})
	require.NoError(t, err)
	assert.Equal(t, 2, result.CommitsScanned)
	assert.Equal(t, 0, result.CommitsModified)
	newTip, _ := repo.Head()
	assert.Equal(t, originalTip.Hash(), newTip.Hash())
}

func TestRewrite_SingleCommitWithPattern(t *testing.T) {
	repo := makeRepo(t, []string{"initial commit\n", "feat: add stuff\n\n" + defaultPattern + "\n"})
	result, err := history.Rewrite(repo, history.Config{Pattern: defaultPattern})
	require.NoError(t, err)
	assert.Equal(t, 2, result.CommitsScanned)
	assert.Equal(t, 1, result.CommitsModified)
	msgs := commitMessages(t, repo)
	assert.Equal(t, "feat: add stuff\n\n", msgs[0])
}

func TestRewrite_MultipleCommitsWithPattern(t *testing.T) {
	repo := makeRepo(t, []string{"initial commit\n", "feat: first\n\n" + defaultPattern + "\n", "feat: second\n", "feat: third\n\n" + defaultPattern + "\n"})
	result, err := history.Rewrite(repo, history.Config{Pattern: defaultPattern})
	require.NoError(t, err)
	assert.Equal(t, 2, result.CommitsModified)
	msgs := commitMessages(t, repo)
	assert.Equal(t, "feat: third\n\n", msgs[0])
	assert.Equal(t, "feat: first\n\n", msgs[2])
}

func TestRewrite_PreservesCommitMetadata(t *testing.T) {
	targetMsg := "feat: my feature\n\n" + defaultPattern + "\n"
	repo := makeRepo(t, []string{"initial commit\n", targetMsg})
	ref, err := repo.Head()
	require.NoError(t, err)
	origCommit, err := repo.CommitObject(ref.Hash())
	require.NoError(t, err)
	_, err = history.Rewrite(repo, history.Config{Pattern: defaultPattern})
	require.NoError(t, err)
	ref, err = repo.Head()
	require.NoError(t, err)
	newCommit, err := repo.CommitObject(ref.Hash())
	require.NoError(t, err)
	assert.Equal(t, origCommit.Author.Name, newCommit.Author.Name)
	assert.Equal(t, origCommit.TreeHash, newCommit.TreeHash)
	assert.Equal(t, origCommit.MergeTag, newCommit.MergeTag)
	assert.Equal(t, origCommit.Encoding, newCommit.Encoding)
	assert.Empty(t, newCommit.PGPSignature)
}

func TestRewrite_DryRun_DoesNotModifyRepo(t *testing.T) {
	repo := makeRepo(t, []string{"initial commit\n", "feat: add stuff\n\n" + defaultPattern + "\n"})
	originalTip, _ := repo.Head()
	result, err := history.Rewrite(repo, history.Config{Pattern: defaultPattern, DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, 1, result.CommitsModified)
	newTip, _ := repo.Head()
	assert.Equal(t, originalTip.Hash(), newTip.Hash())
}

func TestRewrite_ClearsPGPSignatureOnRewrite(t *testing.T) {
	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	require.NoError(t, err)
	sig := object.Signature{Name: "Test User", Email: "test@example.com", When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	emptyTreeHash, err := storeEmptyTree(repo)
	require.NoError(t, err)
	commit := &object.Commit{Author: sig, Committer: sig, MergeTag: "object deadbeef\n\ntag v1.0.0\n", PGPSignature: "-----BEGIN PGP SIGNATURE-----\nabc\n-----END PGP SIGNATURE-----", Encoding: object.MessageEncoding("ISO-8859-1"), Message: "feat: signed commit\n\n" + defaultPattern + "\n", TreeHash: emptyTreeHash}
	obj := store.NewEncodedObject()
	require.NoError(t, commit.Encode(obj))
	hash, err := store.SetEncodedObject(obj)
	require.NoError(t, err)
	ref := plumbing.NewHashReference("refs/heads/main", hash)
	require.NoError(t, store.SetReference(ref))
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
	require.NoError(t, store.SetReference(headRef))
	original, err := repo.CommitObject(hash)
	require.NoError(t, err)
	_, err = history.Rewrite(repo, history.Config{Pattern: defaultPattern})
	require.NoError(t, err)
	newRef, err := repo.Head()
	require.NoError(t, err)
	rewritten, err := repo.CommitObject(newRef.Hash())
	require.NoError(t, err)
	assert.Equal(t, original.MergeTag, rewritten.MergeTag)
	assert.Equal(t, original.Encoding, rewritten.Encoding)
	assert.Empty(t, rewritten.PGPSignature)
}

func TestRewrite_RewritesExactMatchIdentity(t *testing.T) {
	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	require.NoError(t, err)
	emptyTreeHash, err := storeEmptyTree(repo)
	require.NoError(t, err)
	sig := object.Signature{Name: "copilot-swe-agent[bot]", Email: "198982749+Copilot@users.noreply.github.com", When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	commit := &object.Commit{Author: sig, Committer: sig, PGPSignature: "-----BEGIN PGP SIGNATURE-----\nabc\n-----END PGP SIGNATURE-----\n", Message: "feat: identity rewrite\n", TreeHash: emptyTreeHash}
	obj := store.NewEncodedObject()
	require.NoError(t, commit.Encode(obj))
	hash, err := store.SetEncodedObject(obj)
	require.NoError(t, err)
	ref := plumbing.NewHashReference("refs/heads/main", hash)
	require.NoError(t, store.SetReference(ref))
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
	require.NoError(t, store.SetReference(headRef))
	cfg := history.Config{IdentityRewrite: &history.IdentityRewrite{From: history.Identity{Name: "copilot-swe-agent[bot]", Email: "198982749+Copilot@users.noreply.github.com"}, To: history.Identity{Name: "gpablo6", Email: "gpablo6@outlook.com"}, RewriteAuthor: true, RewriteCommitter: true}}
	result, err := history.Rewrite(repo, cfg)
	require.NoError(t, err)
	assert.Equal(t, 1, result.CommitsModified)
	newHead, err := repo.Head()
	require.NoError(t, err)
	rewritten, err := repo.CommitObject(newHead.Hash())
	require.NoError(t, err)
	assert.Equal(t, "gpablo6", rewritten.Author.Name)
	assert.Equal(t, "gpablo6@outlook.com", rewritten.Author.Email)
	assert.Empty(t, rewritten.PGPSignature)
}

func TestRewrite_IdentityRewrite_RequiresExactMatch(t *testing.T) {
	repo := makeRepo(t, []string{"initial commit\n"})
	origHead, err := repo.Head()
	require.NoError(t, err)
	cfg := history.Config{IdentityRewrite: &history.IdentityRewrite{From: history.Identity{Name: "Different User", Email: "different@example.com"}, To: history.Identity{Name: "gpablo6", Email: "gpablo6@outlook.com"}, RewriteAuthor: true}}
	result, err := history.Rewrite(repo, cfg)
	require.NoError(t, err)
	assert.Equal(t, 0, result.CommitsModified)
	newHead, err := repo.Head()
	require.NoError(t, err)
	assert.Equal(t, origHead.Hash(), newHead.Hash())
}

func TestRewrite_MaxCommits_LimitsTraversal(t *testing.T) {
	repo := makeRepo(t, []string{"initial commit\n\n" + defaultPattern + "\n", "feat: middle\n", "feat: latest\n"})
	originalTip, _ := repo.Head()
	result, err := history.Rewrite(repo, history.Config{Pattern: defaultPattern, MaxCommits: 2})
	require.NoError(t, err)
	assert.Equal(t, 2, result.CommitsScanned)
	assert.Equal(t, 0, result.CommitsModified)
	newTip, _ := repo.Head()
	assert.Equal(t, originalTip.Hash(), newTip.Hash())
}

func TestRewrite_MergeCommitParentsAreRewritten(t *testing.T) {
	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	require.NoError(t, err)
	sig := object.Signature{Name: "Test User", Email: "test@example.com", When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	emptyTreeHash, err := storeEmptyTree(repo)
	require.NoError(t, err)
	writeCommit := func(msg string, parents ...plumbing.Hash) plumbing.Hash {
		commit := &object.Commit{Author: sig, Committer: sig, Message: msg, TreeHash: emptyTreeHash, ParentHashes: parents}
		obj := store.NewEncodedObject()
		require.NoError(t, commit.Encode(obj))
		hash, err := store.SetEncodedObject(obj)
		require.NoError(t, err)
		return hash
	}
	root := writeCommit("root\n")
	left := writeCommit("left\n\n"+defaultPattern+"\n", root)
	right := writeCommit("right\n", root)
	merge := writeCommit("merge branch\n", left, right)
	ref := plumbing.NewHashReference("refs/heads/main", merge)
	require.NoError(t, store.SetReference(ref))
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
	require.NoError(t, store.SetReference(headRef))
	result, err := history.Rewrite(repo, history.Config{Pattern: defaultPattern})
	require.NoError(t, err)
	assert.Equal(t, 4, result.CommitsScanned)
	assert.Equal(t, 1, result.CommitsModified)
	newHeadRef, err := repo.Head()
	require.NoError(t, err)
	newMerge, err := repo.CommitObject(newHeadRef.Hash())
	require.NoError(t, err)
	require.Len(t, newMerge.ParentHashes, 2)
	assert.NotEqual(t, left, newMerge.ParentHashes[0])
	assert.Equal(t, right, newMerge.ParentHashes[1])
}
