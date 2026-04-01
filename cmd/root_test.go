package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gpablo6/p3c/internal/history"
	"github.com/gpablo6/p3c/internal/workflow"
)

func TestMessageClean_RemovesBackupByDefault(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	_, _, err := executeRootCmd(t, "message", "clean")
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)
	c, err := repo.CommitObject(head.Hash())
	require.NoError(t, err)
	assert.Equal(t, "feat: add feature\n\n", c.Message)

	refs, err := defaultBackupService().List(repoDir)
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestMessageClean_KeepBackupFlagRetainsBackupRef(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)
	nowUTC = func() time.Time {
		return time.Date(2026, 3, 18, 21, 15, 16, 0, time.UTC)
	}

	origHead, err := repo.Head()
	require.NoError(t, err)

	_, _, err = executeRootCmd(t, "message", "clean", "--keep-backup")
	require.NoError(t, err)

	refs, err := defaultBackupService().List(repoDir)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, origHead.Hash(), refs[0].Hash())
}

func TestMessageClean_ReportsTemporaryBackupReference(t *testing.T) {
	repoDir, _ := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	stdout, _, err := executeRootCmd(t, "message", "clean")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Created temporary backup reference:")
}

func TestMessageClean_DryRunDoesNotCreateBackup(t *testing.T) {
	repoDir, _ := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	_, _, err := executeRootCmd(t, "message", "clean", "--dry-run")
	require.NoError(t, err)

	refs, err := defaultBackupService().List(repoDir)
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestMessageClean_RestoresHeadWhenRewriteFails(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	origHead, err := repo.Head()
	require.NoError(t, err)

	runHistoryRewrite = func(_ *git.Repository, _ history.Config) (*history.Result, error) {
		return nil, errors.New("forced failure")
	}

	_, _, err = executeRootCmd(t, "message", "clean")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced failure")

	newHead, err := repo.Head()
	require.NoError(t, err)
	assert.Equal(t, origHead.Hash(), newHead.Hash())
}

func TestMessageClean_GCOnFailureFlagRunsGC(t *testing.T) {
	repoDir, _ := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	called := 0
	runHistoryRewrite = func(_ *git.Repository, _ history.Config) (*history.Result, error) {
		return nil, errors.New("forced failure")
	}
	runGitGC = func(_ *git.Repository) error {
		called++
		return nil
	}

	_, _, err := executeRootCmd(t, "message", "clean", "--gc-on-failure")
	require.Error(t, err)
	assert.Equal(t, 1, called)
}

func TestMessageClean_GCAfterRunFlagRunsGC(t *testing.T) {
	repoDir, _ := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	called := 0
	runGitGC = func(_ *git.Repository) error {
		called++
		return nil
	}

	_, _, err := executeRootCmd(t, "message", "clean", "--gc-after-run")
	require.NoError(t, err)
	assert.Equal(t, 1, called)
}

func TestBackupClean_RemovesOnlyP3CBackupRefs(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	_, _, err := executeRootCmd(t, "message", "clean", "--keep-backup")
	require.NoError(t, err)

	refs, err := defaultBackupService().List(repoDir)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	backupRefName := refs[0].Name()

	headRef, err := repo.Head()
	require.NoError(t, err)
	manual := plumbing.ReferenceName("refs/heads/backup/manual-keep")
	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(manual, headRef.Hash())))

	_, _, err = executeRootCmd(t, "backup", "clean")
	require.NoError(t, err)

	_, err = repo.Storer.Reference(backupRefName)
	require.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	_, err = repo.Storer.Reference(manual)
	require.NoError(t, err)
}

func TestBackupClean_DryRunPreservesRefs(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	_, _, err := executeRootCmd(t, "message", "clean", "--keep-backup")
	require.NoError(t, err)

	refs, err := defaultBackupService().List(repoDir)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	refName := refs[0].Name()

	_, _, err = executeRootCmd(t, "backup", "clean", "--dry-run")
	require.NoError(t, err)

	_, err = repo.Storer.Reference(refName)
	require.NoError(t, err)
}

func TestIdentityRewrite_RewritesBothAuthorAndCommitter(t *testing.T) {
	repoDir, repo, hash := makeRepoWithIdentityCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)
	openRepository = func(string) (*git.Repository, error) { return repo, nil }

	orig, err := repo.CommitObject(hash)
	require.NoError(t, err)
	require.NotEmpty(t, orig.PGPSignature)

	_, _, err = executeRootCmd(t,
		"identity", "rewrite",
		"--from", "copilot-swe-agent[bot] <198982749+Copilot@users.noreply.github.com>",
		"--to", "gpablo6 <gpablo6@outlook.com>",
		"--scope", "both",
		"--keep-backup",
	)
	require.NoError(t, err)

	headRef, err := repo.Head()
	require.NoError(t, err)
	rewritten, err := repo.CommitObject(headRef.Hash())
	require.NoError(t, err)
	assert.Equal(t, "gpablo6", rewritten.Author.Name)
	assert.Equal(t, "gpablo6@outlook.com", rewritten.Author.Email)
	assert.Equal(t, "gpablo6", rewritten.Committer.Name)
	assert.Equal(t, "gpablo6@outlook.com", rewritten.Committer.Email)
	assert.Empty(t, rewritten.PGPSignature)
}

func TestRootHelp_ListsExplicitSubcommands(t *testing.T) {
	resetCmdTestState(t)

	stdout, _, err := executeRootCmd(t, "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "p3c message clean")
	assert.Contains(t, stdout, "p3c identity rewrite")
	assert.Contains(t, stdout, "p3c backup clean")
}

func executeRootCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func defaultBackupService() *workflow.BackupService {
	return &workflow.BackupService{OpenRepository: openRepository}
}

func makeRepoWithPatternCommit(t *testing.T) (string, *git.Repository) {
	t.Helper()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	sig := &object.Signature{
		Name:  "Test User",
		Email: "test@example.com",
		When:  time.Date(2026, 3, 18, 21, 0, 0, 0, time.UTC),
	}

	writeAndCommit := func(name, content, message string) plumbing.Hash {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add(name)
		require.NoError(t, err)
		hash, err := wt.Commit(message, &git.CommitOptions{
			Author:    sig,
			Committer: sig,
		})
		require.NoError(t, err)
		return hash
	}

	_ = writeAndCommit("a.txt", "first", "chore: init\n")
	_ = writeAndCommit("a.txt", "second", "feat: add feature\n\n"+history.DefaultPattern+"\n")
	return dir, repo
}

func makeRepoWithIdentityCommit(t *testing.T) (string, *git.Repository, plumbing.Hash) {
	t.Helper()

	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	require.NoError(t, err)

	sig := object.Signature{
		Name:  "copilot-swe-agent[bot]",
		Email: "198982749+Copilot@users.noreply.github.com",
		When:  time.Date(2026, 3, 18, 21, 0, 0, 0, time.UTC),
	}
	emptyTreeHash, err := storeEmptyTree(repo)
	require.NoError(t, err)

	commit := &object.Commit{
		Author:       sig,
		Committer:    sig,
		PGPSignature: "-----BEGIN PGP SIGNATURE-----\nabc\n-----END PGP SIGNATURE-----\n",
		Message:      "feat: authored by bot\n",
		TreeHash:     emptyTreeHash,
	}
	obj := store.NewEncodedObject()
	require.NoError(t, commit.Encode(obj))
	hash, err := store.SetEncodedObject(obj)
	require.NoError(t, err)

	ref := plumbing.NewHashReference("refs/heads/main", hash)
	require.NoError(t, store.SetReference(ref))
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
	require.NoError(t, store.SetReference(headRef))

	return dir, repo, hash
}

func storeEmptyTree(repo *git.Repository) (plumbing.Hash, error) {
	tree := &object.Tree{}
	obj := repo.Storer.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return repo.Storer.SetEncodedObject(obj)
}

func withCWD(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}

func resetCmdTestState(t *testing.T) {
	t.Helper()
	openRepository = func(wd string) (*git.Repository, error) {
		return git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
	}
	runHistoryRewrite = history.Rewrite
	nowUTC = func() time.Time { return time.Now().UTC() }
	runGitGC = executeGitGC
	t.Cleanup(func() {
		openRepository = func(wd string) (*git.Repository, error) {
			return git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
		}
		runHistoryRewrite = history.Rewrite
		nowUTC = func() time.Time { return time.Now().UTC() }
		runGitGC = executeGitGC
	})
}
