// Package cmd implements the p3c command-line interface using cobra.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"github.com/gpablo6/p3c/internal/cleaner"
)

var (
	flagPattern string
	flagDryRun  bool
	flagVerbose bool
	flagBranch  string
)

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

	repo, err := git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return fmt.Errorf("opening git repository: %w", err)
	}

	cfg := cleaner.Config{
		Pattern: flagPattern,
		DryRun:  flagDryRun,
		Verbose: flagVerbose,
	}

	if flagDryRun {
		cmd.PrintErrln("Dry-run mode – no changes will be written.")
	}

	result, err := cleaner.Clean(repo, cfg)
	if err != nil {
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

	return nil
}

// capitalize returns s with its first rune upper-cased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
