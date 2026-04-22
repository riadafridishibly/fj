package comment

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory *cmdutil.Factory
	ID      string
	Yes     bool
}

func NewCmdDelete(f *cmdutil.Factory, k Kind) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a comment by its ID",
		Example: fmt.Sprintf(`  $ %s delete 12345
  $ %s delete 12345 --yes`, k.CLI, k.CLI),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
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

	id, err := strconv.ParseInt(opts.ID, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid comment id: %s", opts.ID)
	}

	if !opts.Yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete comment #%d? This cannot be undone. (y/N): ", id)
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

	if _, err := client.DeleteIssueComment(repo.Owner, repo.Name, id); err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted comment #%d\n", id)
	return nil
}
