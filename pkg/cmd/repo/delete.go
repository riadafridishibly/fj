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
	DryRun  bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <owner/repo>",
		Short: "Delete a repository",
		Example: `  $ fj repo delete owner/repo --yes
  $ fj repo delete owner/repo --dry-run`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Repo = args[0]
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

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Resolve the repo first so a wrong name fails before anything is deleted.
	info, _, err := client.GetRepo(repo.Owner, repo.Name)
	if err != nil {
		return fmt.Errorf("fetching repository %s: %w", repo.FullName(), err)
	}

	fmt.Fprintf(os.Stderr, "Repository %s (%d★)", info.FullName, info.Stars)
	if info.Private {
		fmt.Fprint(os.Stderr, " [private]")
	}
	if info.Archived {
		fmt.Fprint(os.Stderr, " [archived]")
	}
	fmt.Fprintln(os.Stderr)

	if !perform {
		fmt.Fprintln(os.Stderr, "(dry-run; no changes were made)")
		return nil
	}

	if _, err := client.DeleteRepo(repo.Owner, repo.Name); err != nil {
		return fmt.Errorf("deleting repository: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted repository %s\n", repo.FullName())
	return nil
}
