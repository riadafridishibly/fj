package issue

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory *cmdutil.Factory
	Number  string
	Yes     bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <number>",
		Short: "Delete an issue",
		Example: `  $ fj issue delete 42
  $ fj issue delete 42 --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
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

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid issue number: %s", opts.Number)
	}

	if !opts.Yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete issue #%d? This cannot be undone. (y/N): ", index)
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

	_, err = client.DeleteIssue(repo.Owner, repo.Name, index)
	if err != nil {
		return fmt.Errorf("deleting issue: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted issue #%d\n", index)
	return nil
}
