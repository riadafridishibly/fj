package repo

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory *cmdutil.Factory
	Repo    string
	Yes     bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <owner/repo>",
		Short: "Delete a repository",
		Example: `  $ fj repo delete owner/repo
  $ fj repo delete owner/repo --yes`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Repo = args[0]
			return deleteRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "Skip confirmation prompt")

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	repo, err := cmdutil.RepoFromFullName(opts.Repo)
	if err != nil {
		return err
	}

	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}
	_, host, err := cfg.DefaultHost()
	if err != nil {
		return err
	}
	repo.Host = host

	if !opts.Yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete %s? This cannot be undone. (y/N): ", repo.FullName())
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

	_, err = client.DeleteRepo(repo.Owner, repo.Name)
	if err != nil {
		return fmt.Errorf("deleting repository: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted repository %s\n", repo.FullName())
	return nil
}
