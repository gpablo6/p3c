package workflow

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupService_ListAndCleanOnlyP3CRefs(t *testing.T) {
	repo, err := git.Init(memoryRepo(), nil)
	require.NoError(t, err)

	p3cRef := plumbing.NewHashReference("refs/heads/backup/p3c-main-test", plumbing.NewHash("1234567890123456789012345678901234567890"))
	manualRef := plumbing.NewHashReference("refs/heads/backup/manual", plumbing.NewHash("abcdefabcdefabcdefabcdefabcdefabcdefabcd"))
	require.NoError(t, repo.Storer.SetReference(p3cRef))
	require.NoError(t, repo.Storer.SetReference(manualRef))

	svc := &BackupService{OpenRepository: func(string) (*git.Repository, error) { return repo, nil }}

	refs, err := svc.List(".")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, p3cRef.Name(), refs[0].Name())

	count, err := svc.Clean(".", false)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	_, err = repo.Storer.Reference(p3cRef.Name())
	require.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	_, err = repo.Storer.Reference(manualRef.Name())
	require.NoError(t, err)
}

func TestBackupService_CleanDryRunKeepsRefs(t *testing.T) {
	repo, err := git.Init(memoryRepo(), nil)
	require.NoError(t, err)

	p3cRef := plumbing.NewHashReference("refs/heads/backup/p3c-main-test", plumbing.NewHash("1234567890123456789012345678901234567890"))
	require.NoError(t, repo.Storer.SetReference(p3cRef))

	svc := &BackupService{OpenRepository: func(string) (*git.Repository, error) { return repo, nil }}

	count, err := svc.Clean(".", true)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	_, err = repo.Storer.Reference(p3cRef.Name())
	require.NoError(t, err)
}
