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
	DryRun     bool
	CleanupTag bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <tag>",
		Short: "Delete a release",
		Example: `  $ fj release delete v1.2.0 --yes
  $ fj release delete v1.2.0 --dry-run
  $ fj release delete v1.2.0 --yes --cleanup-tag`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Tag = args[0]
			return deleteRun(opts)
		},
	}

	cmdutil.AddDeleteFlags(cmd, &opts.Yes, &opts.DryRun)
	cmd.Flags().BoolVar(&opts.CleanupTag, "cleanup-tag", false, "Also delete the git tag")

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

	// Resolve the release first so a wrong tag fails before anything is deleted.
	rel, _, err := client.GetReleaseByTag(repo.Owner, repo.Name, opts.Tag)
	if err != nil {
		return fmt.Errorf("fetching release %q: %w", opts.Tag, err)
	}

	fmt.Fprintf(os.Stderr, "Release #%d %q (tag %s)", rel.ID, rel.Title, opts.Tag)
	if rel.IsDraft {
		fmt.Fprint(os.Stderr, " [draft]")
	}
	if rel.IsPrerelease {
		fmt.Fprint(os.Stderr, " [prerelease]")
	}
	fmt.Fprintln(os.Stderr)
	if opts.CleanupTag {
		fmt.Fprintf(os.Stderr, "Git tag %q will also be deleted\n", opts.Tag)
	}

	if !perform {
		fmt.Fprintln(os.Stderr, cmdutil.DryRunMessage)
		return nil
	}

	if opts.CleanupTag {
		if _, err := client.DeleteReleaseByTag(repo.Owner, repo.Name, opts.Tag); err != nil {
			return fmt.Errorf("deleting release: %w", err)
		}
		if _, err := client.DeleteTag(repo.Owner, repo.Name, opts.Tag); err != nil {
			return fmt.Errorf("deleting tag: %w", err)
		}
	} else {
		if _, err := client.DeleteRelease(repo.Owner, repo.Name, rel.ID); err != nil {
			return fmt.Errorf("deleting release: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted release %s\n", opts.Tag)
	return nil
}
