package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/gpablo6/p3c/internal/history"
)

type OpenRepositoryFunc func(string) (*git.Repository, error)
type RunHistoryRewriteFunc func(*git.Repository, history.Config) (*history.Result, error)
type RunGitGCFunc func(*git.Repository) error

// RewriteOptions controls the operational workflow around a history rewrite.
type RewriteOptions struct {
	DryRun     bool
	KeepBackup bool
	GCOnFail   bool
	GCAfter    bool
}

// RewriteService orchestrates repository opening, backup management, rewrite
// execution, rollback, and optional prune behavior.
type RewriteService struct {
	OpenRepository    OpenRepositoryFunc
	RunHistoryRewrite RunHistoryRewriteFunc
	RunGitGC          RunGitGCFunc
	NowUTC            func() time.Time
}

// RewriteResult reports both the rewrite summary and any backup refs involved in
// the operation lifecycle.
type RewriteResult struct {
	Result                *history.Result
	CreatedBackupRefName  plumbing.ReferenceName
	RetainedBackupRefName plumbing.ReferenceName
}

// Run performs a full rewrite workflow for the current HEAD branch in repoPath.
func (s *RewriteService) Run(repoPath string, cfg history.Config, opts RewriteOptions) (*RewriteResult, error) {
	repo, err := s.OpenRepository(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening git repository: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD before backup: %w", err)
	}

	var backupRefName plumbing.ReferenceName
	if !opts.DryRun {
		backupRefName, err = s.createBackupRef(repo, headRef)
		if err != nil {
			return nil, fmt.Errorf("creating backup reference: %w", err)
		}
	}

	result, err := s.RunHistoryRewrite(repo, cfg)
	if err != nil {
		if !opts.DryRun {
			restoreRef := plumbing.NewHashReference(headRef.Name(), headRef.Hash())
			if restoreErr := repo.Storer.SetReference(restoreRef); restoreErr != nil {
				return nil, fmt.Errorf("cleaning history: %w (rollback failed: %v; backup: %s)", err, restoreErr, backupRefName.String())
			}
		}
		if opts.GCOnFail {
			if gcErr := s.RunGitGC(repo); gcErr != nil {
				return &RewriteResult{Result: result, CreatedBackupRefName: backupRefName, RetainedBackupRefName: backupRefName}, fmt.Errorf("cleaning history: %w (gc failed: %v)", err, gcErr)
			}
		}
		return &RewriteResult{Result: result, CreatedBackupRefName: backupRefName, RetainedBackupRefName: backupRefName}, fmt.Errorf("cleaning history: %w", err)
	}

	if !opts.DryRun {
		if !opts.KeepBackup {
			if err := repo.Storer.RemoveReference(backupRefName); err != nil {
				return &RewriteResult{Result: result, CreatedBackupRefName: backupRefName}, fmt.Errorf("removing backup reference %s: %w", backupRefName, err)
			}
		}
		if opts.GCAfter {
			if err := s.RunGitGC(repo); err != nil {
				return &RewriteResult{Result: result, CreatedBackupRefName: backupRefName, RetainedBackupRefName: keepBackupRefName(opts.KeepBackup, backupRefName)}, fmt.Errorf("pruning unreachable objects after successful run: %w", err)
			}
		}
	}

	return &RewriteResult{
		Result:                result,
		CreatedBackupRefName:  backupRefName,
		RetainedBackupRefName: keepBackupRefName(opts.KeepBackup, backupRefName),
	}, nil
}

func keepBackupRefName(keep bool, name plumbing.ReferenceName) plumbing.ReferenceName {
	if !keep {
		return ""
	}
	return name
}

func (s *RewriteService) createBackupRef(repo *git.Repository, headRef *plumbing.Reference) (plumbing.ReferenceName, error) {
	branch := headRef.Name().Short()
	branch = strings.ReplaceAll(branch, "/", "-")
	timestamp := s.NowUTC().Format("20060102-150405")
	shortHash := headRef.Hash().String()
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	base := fmt.Sprintf("refs/heads/backup/p3c-%s-%s-%s", branch, timestamp, shortHash)
	for i := 0; ; i++ {
		name := plumbing.ReferenceName(base)
		if i > 0 {
			name = plumbing.ReferenceName(fmt.Sprintf("%s-%d", base, i+1))
		}
		_, err := repo.Storer.Reference(name)
		if err == nil {
			continue
		}
		if err != nil && err != plumbing.ErrReferenceNotFound {
			return "", err
		}
		ref := plumbing.NewHashReference(name, headRef.Hash())
		if err := repo.Storer.SetReference(ref); err != nil {
			return "", err
		}
		return name, nil
	}
}
