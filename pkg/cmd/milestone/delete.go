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
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <milestone>",
		Short: "Delete a milestone",
		Example: `  $ fj milestone delete v1.0
  $ fj milestone delete v1.0 --yes`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return deleteRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "Skip confirmation prompt")

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	if !opts.Yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete milestone %q? This cannot be undone. (y/N): ", opts.Name)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	if _, err := client.DeleteMilestoneByName(repo.Owner, repo.Name, opts.Name); err != nil {
		return fmt.Errorf("deleting milestone: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted milestone %q\n", opts.Name)
	return nil
}
