// Package cmd implements the p3c command-line interface using cobra.
package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"

	"github.com/gpablo6/p3c/internal/cleaner"
)

var (
	flagPattern string
	flagDryRun  bool
	flagVerbose bool
	flagBranch  string
	flagMax     int
	flagKeepBak bool
	flagGCFail  bool
	flagGCAfter bool
)

var (
	// Injectable hooks keep run() behavior deterministic in tests without
	// changing production semantics. Defaults always point to real impls.
	openRepository = func(wd string) (*git.Repository, error) {
		return git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{
			DetectDotGit: true,
		})
	}
	runCleaner = cleaner.Clean
	nowUTC     = func() time.Time { return time.Now().UTC() }
	runGitGC   = executeGitGC
)

func executeGitGC(repo *git.Repository) error {
	// We intentionally use go-git prune instead of shelling out to `git gc`.
	// The tool's goal is to avoid leaving loose unreachable objects after a
	// rewrite, not to run full repository maintenance/repacking.
	return repo.Prune(git.PruneOptions{Handler: repo.DeleteObject})
}

// rootCmd is the top-level cobra command for p3c.
var rootCmd = &cobra.Command{
	Use:   "p3c [flags]",
	Short: "Pesky Claude Code Co-Author – scrub AI co-author trailers from git history",
	Long: `p3c rewrites the commit history of the current git repository,
removing every occurrence of a specified line from commit messages.

By default it targets the Claude Opus co-author trailer:

    Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>

Run p3c from inside any git repository on the branch you want to clean.
It rewrites history in place; use --dry-run to preview changes first.

WARNING: this rewrites history. Force-push is required to update remote
branches. Coordinate with collaborators before running on shared branches.`,
	RunE: run,
}

// Execute is the entry point called by main.  It sets up cobra and runs the
// root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// cobra already prints the error; just exit with a non-zero code.
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(
		&flagPattern, "pattern", "p",
		cleaner.DefaultPattern,
		"Exact line to remove from commit messages",
	)
	rootCmd.Flags().BoolVarP(
		&flagDryRun, "dry-run", "n",
		false,
		"Show what would be changed without modifying the repository",
	)
	rootCmd.Flags().BoolVarP(
		&flagVerbose, "verbose", "v",
		false,
		"Print each commit that is rewritten",
	)
	rootCmd.Flags().IntVar(
		&flagMax, "max-commits",
		0,
		"Limit traversal to the most recent N commits (default: all)",
	)
	rootCmd.Flags().BoolVar(
		&flagKeepBak, "keep-backup",
		false,
		"Keep the temporary backup branch after a successful rewrite",
	)
	rootCmd.Flags().BoolVar(
		&flagGCFail, "gc-on-failure",
		false,
		"Prune unreachable loose objects if cleaning fails after creating rewritten objects",
	)
	rootCmd.Flags().BoolVar(
		&flagGCAfter, "gc-after-run",
		false,
		"Prune unreachable loose objects after a successful run",
	)
	// --branch is reserved for future use; currently p3c always operates on
	// the checked-out HEAD branch.
	rootCmd.Flags().StringVarP(
		&flagBranch, "branch", "b",
		"",
		"Branch to clean (default: currently checked-out branch)",
	)
	// Mark --branch hidden until it is fully implemented.
	if err := rootCmd.Flags().MarkHidden("branch"); err != nil {
		// MarkHidden can only fail if the flag doesn't exist, which is a
		// programming error, so panic is appropriate here.
		panic(err)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	// Open the repository in the current working directory.
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	repo, err := openRepository(wd)
	if err != nil {
		return fmt.Errorf("opening git repository: %w", err)
	}

	cfg := cleaner.Config{
		Pattern:    flagPattern,
		DryRun:     flagDryRun,
		Verbose:    flagVerbose,
		MaxCommits: flagMax,
	}

	if cfg.MaxCommits < 0 {
		return fmt.Errorf("--max-commits must be >= 0")
	}

	if flagDryRun {
		cmd.PrintErrln("Dry-run mode – no changes will be written.")
	}

	var backupRefName plumbing.ReferenceName
	headRef, err := repo.Head()
	if err != nil {
		return fmt.Errorf("resolving HEAD before backup: %w", err)
	}
	if !flagDryRun {
		backupRefName, err = createBackupRef(repo, headRef)
		if err != nil {
			return fmt.Errorf("creating backup reference: %w", err)
		}
		cmd.Printf("Created temporary backup reference: %s\n", backupRefName.String())
	}

	result, err := runCleaner(repo, cfg)
	if err != nil {
		if !flagDryRun {
			// If cleaning failed, restore the original branch tip so users are not
			// left on a partially rewritten ref.
			restoreRef := plumbing.NewHashReference(headRef.Name(), headRef.Hash())
			if restoreErr := repo.Storer.SetReference(restoreRef); restoreErr != nil {
				return fmt.Errorf(
					"cleaning history: %w (rollback failed: %v; backup: %s)",
					err, restoreErr, backupRefName.String(),
				)
			}
			cmd.Printf("Restored %s to backup tip %s\n", headRef.Name().String(), headRef.Hash())
		}
		if flagGCFail {
			if gcErr := runGitGC(repo); gcErr != nil {
				cmd.Printf("Warning: failed to prune unreachable objects after failure: %v\n", gcErr)
			} else {
				cmd.Println("Pruned unreachable objects after failure.")
			}
		}
		return fmt.Errorf("cleaning history: %w", err)
	}

	verb := "cleaned"
	if flagDryRun {
		verb = "would clean"
	}

	cmd.Printf(
		"Scanned %d commit(s). %s %d commit message(s).\n",
		result.CommitsScanned,
		capitalize(verb),
		result.CommitsModified,
	)

	if result.CommitsModified > 0 && !flagDryRun {
		cmd.Println("\nRemember to force-push to update any remote branches:")
		cmd.Println("  git push --force-with-lease")
	}
	if !flagDryRun {
		if flagKeepBak {
			cmd.Printf("Backup retained at: %s\n", backupRefName.String())
		} else {
			// On successful runs we remove the temporary backup by default to avoid
			// cluttering refs. Users can opt out via --keep-backup.
			if err := repo.Storer.RemoveReference(backupRefName); err != nil {
				cmd.Printf("Warning: failed to remove backup reference %s: %v\n", backupRefName.String(), err)
			}
		}
		if flagGCAfter {
			if err := runGitGC(repo); err != nil {
				return fmt.Errorf("pruning unreachable objects after successful run: %w", err)
			}
			cmd.Println("Pruned unreachable objects after successful run.")
		}
	}

	return nil
}

func createBackupRef(repo *git.Repository, headRef *plumbing.Reference) (plumbing.ReferenceName, error) {
	branch := headRef.Name().Short()
	branch = strings.ReplaceAll(branch, "/", "-")
	timestamp := nowUTC().Format("20060102-150405")
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

// capitalize returns s with its first rune upper-cased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
