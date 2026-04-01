package workflow

import (
	"errors"
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

func TestRewriteService_RunDryRunSkipsBackupCreation(t *testing.T) {
	repo := makeWorkflowRepo(t)
	svc := testRewriteService(repo)

	result, err := svc.Run(".", history.Config{Pattern: history.DefaultPattern, DryRun: true}, RewriteOptions{DryRun: true})
	require.NoError(t, err)
	assert.Empty(t, result.CreatedBackupRefName)

	refs, err := (&BackupService{OpenRepository: func(string) (*git.Repository, error) { return repo, nil }}).List(".")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestRewriteService_RunKeepBackupRetainsRef(t *testing.T) {
	repo := makeWorkflowRepo(t)
	svc := testRewriteService(repo)

	result, err := svc.Run(".", history.Config{Pattern: history.DefaultPattern}, RewriteOptions{KeepBackup: true})
	require.NoError(t, err)
	assert.NotEmpty(t, result.CreatedBackupRefName)
	assert.Equal(t, result.CreatedBackupRefName, result.RetainedBackupRefName)

	_, err = repo.Storer.Reference(result.RetainedBackupRefName)
	require.NoError(t, err)
}

func TestRewriteService_RunFailureRestoresHeadAndKeepsBackup(t *testing.T) {
	repo := makeWorkflowRepo(t)
	origHead, err := repo.Head()
	require.NoError(t, err)

	svc := testRewriteService(repo)
	svc.RunHistoryRewrite = func(_ *git.Repository, _ history.Config) (*history.Result, error) {
		return nil, errors.New("forced failure")
	}

	result, err := svc.Run(".", history.Config{Pattern: history.DefaultPattern}, RewriteOptions{})
	require.Error(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.CreatedBackupRefName)

	newHead, err := repo.Head()
	require.NoError(t, err)
	assert.Equal(t, origHead.Hash(), newHead.Hash())
	_, err = repo.Storer.Reference(result.CreatedBackupRefName)
	require.NoError(t, err)
}

func TestRewriteService_CreateBackupRefAddsSuffixOnCollision(t *testing.T) {
	repo := makeWorkflowRepo(t)
	svc := testRewriteService(repo)
	headRef, err := repo.Head()
	require.NoError(t, err)

	first, err := svc.createBackupRef(repo, headRef)
	require.NoError(t, err)
	second, err := svc.createBackupRef(repo, headRef)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.Contains(t, second.String(), "-2")
}

func testRewriteService(repo *git.Repository) *RewriteService {
	return &RewriteService{
		OpenRepository:    func(string) (*git.Repository, error) { return repo, nil },
		RunHistoryRewrite: history.Rewrite,
		RunGitGC:          func(*git.Repository) error { return nil },
		NowUTC: func() time.Time {
			return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
		},
	}
}

func makeWorkflowRepo(t *testing.T) *git.Repository {
	t.Helper()

	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	require.NoError(t, err)

	sig := object.Signature{
		Name:  "Test User",
		Email: "test@example.com",
		When:  time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
	}
	treeHash, err := storeEmptyTree(repo)
	require.NoError(t, err)

	root := &object.Commit{
		Author:    sig,
		Committer: sig,
		Message:   "root\n",
		TreeHash:  treeHash,
	}
	rootObj := store.NewEncodedObject()
	require.NoError(t, root.Encode(rootObj))
	rootHash, err := store.SetEncodedObject(rootObj)
	require.NoError(t, err)

	head := &object.Commit{
		Author:       sig,
		Committer:    sig,
		Message:      "feat: test\n\n" + history.DefaultPattern + "\n",
		TreeHash:     treeHash,
		ParentHashes: []plumbing.Hash{rootHash},
	}
	headObj := store.NewEncodedObject()
	require.NoError(t, head.Encode(headObj))
	headHash, err := store.SetEncodedObject(headObj)
	require.NoError(t, err)

	require.NoError(t, store.SetReference(plumbing.NewHashReference("refs/heads/main", headHash)))
	require.NoError(t, store.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")))
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

func memoryRepo() *memory.Storage {
	return memory.NewStorage()
}
