// Package cmd implements the p3c command-line interface using cobra.
package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	backupcmd "github.com/gpablo6/p3c/cmd/backup"
	identitycmd "github.com/gpablo6/p3c/cmd/identity"
	messagecmd "github.com/gpablo6/p3c/cmd/message"
	"github.com/gpablo6/p3c/internal/history"
	"github.com/gpablo6/p3c/internal/workflow"
)

var (
	openRepository = func(wd string) (*git.Repository, error) {
		return git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
	}
	runHistoryRewrite = history.Rewrite
	nowUTC            = func() time.Time { return time.Now().UTC() }
	runGitGC          = executeGitGC
)

func executeGitGC(repo *git.Repository) error {
	return repo.Prune(git.PruneOptions{Handler: repo.DeleteObject})
}

func currentWorkingDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return wd, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func newRootCmd() *cobra.Command {
	rewriteSvc := &workflow.RewriteService{
		OpenRepository:    openRepository,
		RunHistoryRewrite: runHistoryRewrite,
		RunGitGC:          runGitGC,
		NowUTC:            nowUTC,
	}
	backupSvc := &workflow.BackupService{OpenRepository: openRepository}

	rootCmd := &cobra.Command{
		Use:   "p3c",
		Short: "Rewrite git history with explicit cleanup and identity actions",
		Long: `p3c rewrites git history in the current repository.

All history-changing operations are explicit subcommands:

  p3c message clean
  p3c identity rewrite
  p3c backup clean

WARNING: history rewrites change commit SHAs. Force-push is required to update
remote branches. Coordinate with collaborators before using rewrite commands on
shared branches.`,
	}

	rootCmd.AddCommand(
		messagecmd.NewCommand(rewriteSvc, currentWorkingDir, capitalize),
		identitycmd.NewCommand(rewriteSvc, currentWorkingDir, capitalize),
		backupcmd.NewCommand(backupSvc, currentWorkingDir),
	)
	return rootCmd
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
