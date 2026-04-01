package backup

import "github.com/spf13/cobra"

import "github.com/gpablo6/p3c/internal/workflow"

type cwdFunc func() (string, error)

type cleanFlags struct {
	dryRun  bool
	verbose bool
}

func NewCommand(backupSvc *workflow.BackupService, cwd cwdFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage p3c backup refs",
	}
	cmd.AddCommand(newCleanCmd(backupSvc, cwd))
	return cmd
}

func newCleanCmd(backupSvc *workflow.BackupService, cwd cwdFunc) *cobra.Command {
	flags := &cleanFlags{}
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove local backup refs created by p3c",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoPath, err := cwd()
			if err != nil {
				return err
			}
			refs, err := backupSvc.List(repoPath)
			if err != nil {
				return err
			}
			verb := "Removed"
			if flags.dryRun {
				verb = "Would remove"
				cmd.PrintErrln("Dry-run mode – no backup refs will be removed.")
			}
			for _, ref := range refs {
				if flags.verbose {
					cmd.Printf("%s %s\n", verb, ref.Name().String())
				}
			}
			count, err := backupSvc.Clean(repoPath, flags.dryRun)
			if err != nil {
				return err
			}
			cmd.Printf("%s %d backup ref(s).\n", verb, count)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&flags.dryRun, "dry-run", "n", false, "Show which backup refs would be removed without deleting them")
	cmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "Print each backup ref processed")
	return cmd
}
