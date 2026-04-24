package release

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory    *cmdutil.Factory
	Tag        string
	Yes        bool
	CleanupTag bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <tag>",
		Short: "Delete a release",
		Example: `  $ fj release delete v1.2.0
  $ fj release delete v1.2.0 --yes
  $ fj release delete v1.2.0 --cleanup-tag`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Tag = args[0]
			return deleteRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&opts.CleanupTag, "cleanup-tag", false, "Also delete the git tag")

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	if !opts.Yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete release %s? This cannot be undone. (y/N): ", opts.Tag)
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

	if opts.CleanupTag {
		_, err := client.DeleteReleaseByTag(repo.Owner, repo.Name, opts.Tag)
		if err != nil {
			return fmt.Errorf("deleting release: %w", err)
		}
		if _, err := client.DeleteTag(repo.Owner, repo.Name, opts.Tag); err != nil {
			return fmt.Errorf("deleting tag: %w", err)
		}
	} else {
		rel, _, err := client.GetReleaseByTag(repo.Owner, repo.Name, opts.Tag)
		if err != nil {
			return fmt.Errorf("getting release: %w", err)
		}
		if _, err := client.DeleteRelease(repo.Owner, repo.Name, rel.ID); err != nil {
			return fmt.Errorf("deleting release: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted release %s\n", opts.Tag)
	return nil
}
