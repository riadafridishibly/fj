package milestone

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory *cmdutil.Factory
	Name    string
	Yes     bool
	DryRun  bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <milestone>",
		Short: "Delete a milestone",
		Example: `  $ fj milestone delete v1.0 --yes
  $ fj milestone delete v1.0 --dry-run`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return deleteRun(opts)
		},
	}

	cmdutil.AddDeleteFlags(cmd, &opts.Yes, &opts.DryRun)

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	perform, err := cmdutil.ResolveDeleteFlags(opts.Yes, opts.DryRun)
	if err != nil {
		return err
	}

	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Resolve the milestone first so a wrong name fails before anything is
	// deleted, and so the user can verify the target.
	ms, _, err := client.GetMilestoneByName(repo.Owner, repo.Name, opts.Name)
	if err != nil {
		return fmt.Errorf("fetching milestone %q: %w", opts.Name, err)
	}

	fmt.Fprintf(os.Stderr, "Milestone #%d %q (%s, %d open / %d closed)\n",
		ms.ID, ms.Title, ms.State, ms.OpenIssues, ms.ClosedIssues)

	if !perform {
		fmt.Fprintln(os.Stderr, cmdutil.DryRunMessage)
		return nil
	}

	if _, err := client.DeleteMilestone(repo.Owner, repo.Name, ms.ID); err != nil {
		return fmt.Errorf("deleting milestone: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted milestone %q\n", opts.Name)
	return nil
}
