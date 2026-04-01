package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gpablo6/p3c/internal/cleaner"
)

func TestRun_RemovesBackupByDefault(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	flagPattern = cleaner.DefaultPattern
	flagDryRun = false
	flagVerbose = false
	flagMax = 0
	flagKeepBak = false

	err := run(&cobra.Command{}, nil)
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)
	c, err := repo.CommitObject(head.Hash())
	require.NoError(t, err)
	assert.Equal(t, "feat: add feature\n\n", c.Message)
	assert.Empty(t, listBackupRefs(t, repo))
}

func TestRun_KeepBackupFlagRetainsBackupRef(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)
	nowUTC = func() time.Time {
		return time.Date(2026, 3, 18, 21, 15, 16, 0, time.UTC)
	}

	origHead, err := repo.Head()
	require.NoError(t, err)

	flagPattern = cleaner.DefaultPattern
	flagDryRun = false
	flagVerbose = false
	flagMax = 0
	flagKeepBak = true

	err = run(&cobra.Command{}, nil)
	require.NoError(t, err)

	backups := listBackupRefs(t, repo)
	require.Len(t, backups, 1)
	assert.Equal(t, origHead.Hash(), backups[0].Hash())
}

func TestRun_ReportsTemporaryBackupReference(t *testing.T) {
	repoDir, _ := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	cmd := &cobra.Command{}
	var stdout strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	flagPattern = cleaner.DefaultPattern
	flagDryRun = false

	err := run(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created temporary backup reference:")
}

func TestRun_DryRunDoesNotCreateBackup(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	flagPattern = cleaner.DefaultPattern
	flagDryRun = true
	flagVerbose = false
	flagMax = 0
	flagKeepBak = false

	err := run(&cobra.Command{}, nil)
	require.NoError(t, err)
	assert.Empty(t, listBackupRefs(t, repo))
}

func TestRun_RestoresHeadWhenCleanerFails(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	origHead, err := repo.Head()
	require.NoError(t, err)

	runCleaner = func(_ *git.Repository, _ cleaner.Config) (*cleaner.Result, error) {
		return nil, errors.New("forced failure")
	}

	flagPattern = cleaner.DefaultPattern
	flagDryRun = false
	flagVerbose = false
	flagMax = 0
	flagKeepBak = false

	err = run(&cobra.Command{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced failure")

	newHead, headErr := repo.Head()
	require.NoError(t, headErr)
	assert.Equal(t, origHead.Hash(), newHead.Hash())
	require.Len(t, listBackupRefs(t, repo), 1)
}

func TestRun_GCOnFailureFlagRunsGC(t *testing.T) {
	repoDir, _ := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	called := 0
	runCleaner = func(_ *git.Repository, _ cleaner.Config) (*cleaner.Result, error) {
		return nil, errors.New("forced failure")
	}
	runGitGC = func(_ *git.Repository) error {
		called++
		return nil
	}

	flagPattern = cleaner.DefaultPattern
	flagDryRun = false
	flagGCFail = true

	err := run(&cobra.Command{}, nil)
	require.Error(t, err)
	assert.Equal(t, 1, called)
}

func TestRun_GCAfterRunFlagRunsGC(t *testing.T) {
	repoDir, _ := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)

	called := 0
	runGitGC = func(_ *git.Repository) error {
		called++
		return nil
	}

	flagPattern = cleaner.DefaultPattern
	flagDryRun = false
	flagGCAfter = true

	err := run(&cobra.Command{}, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, called)
}

func TestCreateBackupRef_AvoidsNameCollisions(t *testing.T) {
	repoDir, repo := makeRepoWithPatternCommit(t)
	withCWD(t, repoDir)
	resetCmdTestState(t)
	nowUTC = func() time.Time {
		return time.Date(2026, 3, 18, 21, 15, 16, 0, time.UTC)
	}

	headRef, err := repo.Head()
	require.NoError(t, err)

	first, err := createBackupRef(repo, headRef)
	require.NoError(t, err)
	second, err := createBackupRef(repo, headRef)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.True(t, strings.HasPrefix(first.String(), "refs/heads/backup/p3c-"))
	assert.True(t, strings.HasPrefix(second.String(), first.String()+"-") || strings.HasPrefix(second.String(), "refs/heads/backup/p3c-"))

	_, err = repo.Storer.Reference(first)
	require.NoError(t, err)
	_, err = repo.Storer.Reference(second)
	require.NoError(t, err)
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
		wt, wtErr := repo.Worktree()
		require.NoError(t, wtErr)
		_, addErr := wt.Add(name)
		require.NoError(t, addErr)
		hash, commitErr := wt.Commit(message, &git.CommitOptions{
			Author:    sig,
			Committer: sig,
		})
		require.NoError(t, commitErr)
		return hash
	}

	_ = writeAndCommit("a.txt", "first", "chore: init\n")
	_ = writeAndCommit(
		"a.txt",
		"second",
		"feat: add feature\n\n"+cleaner.DefaultPattern+"\n",
	)

	return dir, repo
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
	runCleaner = cleaner.Clean
	nowUTC = func() time.Time { return time.Now().UTC() }
	runGitGC = executeGitGC
	flagPattern = cleaner.DefaultPattern
	flagDryRun = false
	flagVerbose = false
	flagMax = 0
	flagKeepBak = false
	flagGCFail = false
	flagGCAfter = false
	t.Cleanup(func() {
		openRepository = func(wd string) (*git.Repository, error) {
			return git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
		}
		runCleaner = cleaner.Clean
		nowUTC = func() time.Time { return time.Now().UTC() }
		runGitGC = executeGitGC
		flagPattern = cleaner.DefaultPattern
		flagDryRun = false
		flagVerbose = false
		flagMax = 0
		flagKeepBak = false
		flagGCFail = false
		flagGCAfter = false
	})
}

func listBackupRefs(t *testing.T, repo *git.Repository) []*plumbing.Reference {
	t.Helper()
	iter, err := repo.Storer.IterReferences()
	require.NoError(t, err)

	var out []*plumbing.Reference
	require.NoError(t, iter.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(ref.Name().String(), "refs/heads/backup/") {
			out = append(out, ref)
		}
		return nil
	}))
	return out
}
