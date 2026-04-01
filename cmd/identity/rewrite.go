package identity

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gpablo6/p3c/internal/history"
	"github.com/gpablo6/p3c/internal/workflow"
)

type cwdFunc func() (string, error)
type capitalizeFunc func(string) string

type rewriteFlags struct {
	from       string
	fromName   string
	fromEmail  string
	to         string
	toName     string
	toEmail    string
	scope      string
	dryRun     bool
	verbose    bool
	maxCommits int
	keepBackup bool
	gcOnFail   bool
	gcAfter    bool
}

func NewCommand(rewriteSvc *workflow.RewriteService, cwd cwdFunc, capitalize capitalizeFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Rewrite author and committer identities in commit history",
	}
	cmd.AddCommand(newRewriteCmd(rewriteSvc, cwd, capitalize))
	return cmd
}

func newRewriteCmd(rewriteSvc *workflow.RewriteService, cwd cwdFunc, capitalize capitalizeFunc) *cobra.Command {
	flags := &rewriteFlags{scope: "both"}
	cmd := &cobra.Command{
		Use:   "rewrite",
		Short: "Rewrite exact-match author and committer identities",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flags.maxCommits < 0 {
				return fmt.Errorf("--max-commits must be >= 0")
			}
			from, err := buildIdentity(flags.from, flags.fromName, flags.fromEmail, "from")
			if err != nil {
				return err
			}
			to, err := buildIdentity(flags.to, flags.toName, flags.toEmail, "to")
			if err != nil {
				return err
			}
			rewrite := &history.IdentityRewrite{From: from, To: to}
			switch flags.scope {
			case "author":
				rewrite.RewriteAuthor = true
			case "committer":
				rewrite.RewriteCommitter = true
			case "both":
				rewrite.RewriteAuthor = true
				rewrite.RewriteCommitter = true
			default:
				return fmt.Errorf("--scope must be one of: author, committer, both")
			}
			if flags.dryRun {
				cmd.PrintErrln("Dry-run mode – no changes will be written.")
			}
			repoPath, err := cwd()
			if err != nil {
				return err
			}
			result, err := rewriteSvc.Run(repoPath, history.Config{
				DryRun:          flags.dryRun,
				Verbose:         flags.verbose,
				MaxCommits:      flags.maxCommits,
				IdentityRewrite: rewrite,
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
			verb := "rewrote"
			if flags.dryRun {
				verb = "would rewrite"
			}
			cmd.Printf("Scanned %d commit(s). %s %d commit(s).\n", result.Result.CommitsScanned, capitalize(verb), result.Result.CommitsModified)
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
	cmd.Flags().StringVar(&flags.from, "from", "", `Source identity in "Name <email>" format`)
	cmd.Flags().StringVar(&flags.fromName, "from-name", "", "Source identity name for exact matching")
	cmd.Flags().StringVar(&flags.fromEmail, "from-email", "", "Source identity email for exact matching")
	cmd.Flags().StringVar(&flags.to, "to", "", `Target identity in "Name <email>" format`)
	cmd.Flags().StringVar(&flags.toName, "to-name", "", "Target identity name")
	cmd.Flags().StringVar(&flags.toEmail, "to-email", "", "Target identity email")
	cmd.Flags().StringVar(&flags.scope, "scope", "both", "Rewrite scope: author, committer, or both")
	cmd.Flags().BoolVarP(&flags.dryRun, "dry-run", "n", false, "Show what would be changed without modifying the repository")
	cmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "Print each commit that is rewritten")
	cmd.Flags().IntVar(&flags.maxCommits, "max-commits", 0, "Limit traversal to the most recent N commits (default: all)")
	cmd.Flags().BoolVar(&flags.keepBackup, "keep-backup", false, "Keep the temporary backup branch after a successful rewrite")
	cmd.Flags().BoolVar(&flags.gcOnFail, "gc-on-failure", false, "Prune unreachable loose objects if rewriting fails after creating rewritten objects")
	cmd.Flags().BoolVar(&flags.gcAfter, "gc-after-run", false, "Prune unreachable loose objects after a successful run")
	return cmd
}

func buildIdentity(full, name, email, label string) (history.Identity, error) {
	if full != "" && (name != "" || email != "") {
		return history.Identity{}, fmt.Errorf("cannot combine --%s with --%s-name/--%s-email", label, label, label)
	}
	if full != "" {
		return parseIdentity(full, label)
	}
	if name == "" || email == "" {
		return history.Identity{}, fmt.Errorf("--%s requires either --%s \"Name <email>\" or both --%s-name and --%s-email", label, label, label, label)
	}
	return history.Identity{Name: name, Email: email}, nil
}

func parseIdentity(value, label string) (history.Identity, error) {
	value = strings.TrimSpace(value)
	start := strings.LastIndex(value, "<")
	end := strings.LastIndex(value, ">")
	if start <= 0 || end <= start+1 || end != len(value)-1 {
		return history.Identity{}, fmt.Errorf("--%s must use \"Name <email>\" format", label)
	}
	name := strings.TrimSpace(value[:start])
	email := strings.TrimSpace(value[start+1 : end])
	if name == "" || email == "" {
		return history.Identity{}, fmt.Errorf("--%s must use \"Name <email>\" format", label)
	}
	return history.Identity{Name: name, Email: email}, nil
}
