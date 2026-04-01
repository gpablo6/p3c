package message

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gpablo6/p3c/internal/history"
	"github.com/gpablo6/p3c/internal/workflow"
)

type cwdFunc func() (string, error)
type capitalizeFunc func(string) string

type cleanFlags struct {
	pattern    string
	branch     string
	dryRun     bool
	verbose    bool
	maxCommits int
	keepBackup bool
	gcOnFail   bool
	gcAfter    bool
}

func NewCommand(rewriteSvc *workflow.RewriteService, cwd cwdFunc, capitalize capitalizeFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Rewrite commit messages",
	}
	cmd.AddCommand(newCleanCmd(rewriteSvc, cwd, capitalize))
	return cmd
}

func newCleanCmd(rewriteSvc *workflow.RewriteService, cwd cwdFunc, capitalize capitalizeFunc) *cobra.Command {
	flags := &cleanFlags{}
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove an exact-match line from commit messages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flags.maxCommits < 0 {
				return fmt.Errorf("--max-commits must be >= 0")
			}
			if flags.dryRun {
				cmd.PrintErrln("Dry-run mode – no changes will be written.")
			}
			repoPath, err := cwd()
			if err != nil {
				return err
			}
			result, err := rewriteSvc.Run(repoPath, history.Config{
				Pattern:    flags.pattern,
				DryRun:     flags.dryRun,
				Verbose:    flags.verbose,
				MaxCommits: flags.maxCommits,
			}, workflow.RewriteOptions{
				DryRun:     flags.dryRun,
				KeepBackup: flags.keepBackup,
				GCOnFail:   flags.gcOnFail,
				GCAfter:    flags.gcAfter,
			})
			if err != nil {
				return err
			}
			if !flags.dryRun && result.CreatedBackupRefName != "" {
				cmd.Printf("Created temporary backup reference: %s\n", result.CreatedBackupRefName.String())
			}
			verb := "cleaned"
			if flags.dryRun {
				verb = "would clean"
			}
			cmd.Printf("Scanned %d commit(s). %s %d commit message(s).\n", result.Result.CommitsScanned, capitalize(verb), result.Result.CommitsModified)
			if result.Result.CommitsModified > 0 && !flags.dryRun {
				cmd.Println("\nRemember to force-push to update any remote branches:")
				cmd.Println("  git push --force-with-lease")
			}
			if !flags.dryRun && flags.keepBackup && result.RetainedBackupRefName != "" {
				cmd.Printf("Backup retained at: %s\n", result.RetainedBackupRefName.String())
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&flags.pattern, "pattern", "p", history.DefaultPattern, "Exact line to remove from commit messages")
	cmd.Flags().BoolVarP(&flags.dryRun, "dry-run", "n", false, "Show what would be changed without modifying the repository")
	cmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "Print each commit that is rewritten")
	cmd.Flags().IntVar(&flags.maxCommits, "max-commits", 0, "Limit traversal to the most recent N commits (default: all)")
	cmd.Flags().BoolVar(&flags.keepBackup, "keep-backup", false, "Keep the temporary backup branch after a successful rewrite")
	cmd.Flags().BoolVar(&flags.gcOnFail, "gc-on-failure", false, "Prune unreachable loose objects if rewriting fails after creating rewritten objects")
	cmd.Flags().BoolVar(&flags.gcAfter, "gc-after-run", false, "Prune unreachable loose objects after a successful run")
	cmd.Flags().StringVarP(&flags.branch, "branch", "b", "", "Branch to clean (default: currently checked-out branch)")
	if err := cmd.Flags().MarkHidden("branch"); err != nil {
		panic(err)
	}
	return cmd
}
