package issue

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type reopenOptions struct {
	Factory *cmdutil.Factory
	Args    []string
}

func NewCmdReopen(f *cmdutil.Factory) *cobra.Command {
	opts := &reopenOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "reopen <number>",
		Short:   "Reopen an issue",
		Example: `  $ fj issue reopen 42`,
		Args:    cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return reopenRun(opts)
		},
	}

	return cmd
}

func reopenRun(opts *reopenOptions) error {
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

	open := forgejo.StateOpen
	_, _, err = client.EditIssue(repo.Owner, repo.Name, index, forgejo.EditIssueOption{
		State: &open,
	})
	if err != nil {
		return fmt.Errorf("reopening issue: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Reopened issue #%d\n", index)
	return nil
}
