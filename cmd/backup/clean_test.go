package backup

import (
	"bytes"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gpablo6/p3c/internal/workflow"
)

func TestCleanCommand_DryRunVerboseListsRefs(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	ref := plumbing.NewHashReference("refs/heads/backup/p3c-main-test", plumbing.NewHash("1234567890123456789012345678901234567890"))
	require.NoError(t, repo.Storer.SetReference(ref))

	svc := &workflow.BackupService{
		OpenRepository: func(string) (*git.Repository, error) { return repo, nil },
	}
	cmd := NewCommand(svc, func() (string, error) { return dir, nil })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"clean", "--dry-run", "--verbose"})

	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Would remove refs/heads/backup/p3c-main-test")
	assert.Contains(t, stdout.String(), "Would remove 1 backup ref(s).")
	assert.Contains(t, stderr.String(), "Dry-run mode")

	_, err = repo.Storer.Reference(ref.Name())
	require.NoError(t, err)
}

func TestCleanCommand_CleansRefs(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	ref := plumbing.NewHashReference("refs/heads/backup/p3c-main-test", plumbing.NewHash("1234567890123456789012345678901234567890"))
	require.NoError(t, repo.Storer.SetReference(ref))

	svc := &workflow.BackupService{
		OpenRepository: func(string) (*git.Repository, error) { return repo, nil },
	}
	cmd := NewCommand(svc, func() (string, error) { return dir, nil })
	cmd.SetArgs([]string{"clean"})

	err = cmd.Execute()
	require.NoError(t, err)
	_, err = repo.Storer.Reference(ref.Name())
	require.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
}
