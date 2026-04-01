package workflow

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

type BackupService struct {
	OpenRepository OpenRepositoryFunc
}

// List returns local backup refs created by p3c in the target repository.
func (s *BackupService) List(repoPath string) ([]*plumbing.Reference, error) {
	repo, err := s.OpenRepository(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening git repository: %w", err)
	}
	iter, err := repo.Storer.IterReferences()
	if err != nil {
		return nil, err
	}
	var out []*plumbing.Reference
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(ref.Name().String(), "refs/heads/backup/p3c-") {
			out = append(out, ref)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Clean removes local backup refs created by p3c, or just counts them in
// dry-run mode.
func (s *BackupService) Clean(repoPath string, dryRun bool) (int, error) {
	repo, err := s.OpenRepository(repoPath)
	if err != nil {
		return 0, fmt.Errorf("opening git repository: %w", err)
	}
	refs, err := s.List(repoPath)
	if err != nil {
		return 0, fmt.Errorf("listing backup references: %w", err)
	}
	if dryRun {
		return len(refs), nil
	}
	for _, ref := range refs {
		if err := repo.Storer.RemoveReference(ref.Name()); err != nil {
			return 0, fmt.Errorf("removing backup reference %s: %w", ref.Name(), err)
		}
	}
	return len(refs), nil
}
