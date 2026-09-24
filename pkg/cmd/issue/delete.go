package issue

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory *cmdutil.Factory
	Args    []string
	Yes     bool
	DryRun  bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <number>",
		Short: "Delete an issue",
		Example: `  $ fj issue delete 42 --yes
  $ fj issue delete 42 --dry-run`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
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

	index, _, err := opts.Factory.IssueNumber(repo, opts.Args)
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Fetch the issue first so a wrong number fails before anything is deleted,
	// and so the user can verify the target from its title.
	issue, _, err := client.GetIssue(repo.Owner, repo.Name, index)
	if err != nil {
		return fmt.Errorf("fetching issue #%d: %w", index, err)
	}

	author := "unknown"
	if issue.Poster != nil {
		author = issue.Poster.UserName
	}
	fmt.Fprintf(os.Stderr, "Issue #%d %q by %s\n", index, issue.Title, author)

	if !perform {
		fmt.Fprintln(os.Stderr, cmdutil.DryRunMessage)
		return nil
	}

	_, err = client.DeleteIssue(repo.Owner, repo.Name, index)
	if err != nil {
		return fmt.Errorf("deleting issue: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted issue #%d\n", index)
	return nil
}
