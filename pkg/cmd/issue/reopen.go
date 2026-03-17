package issue

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type reopenOptions struct {
	Factory *cmdutil.Factory
	Number  string
}

func NewCmdReopen(f *cmdutil.Factory) *cobra.Command {
	opts := &reopenOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "reopen <number>",
		Short:   "Reopen an issue",
		Example: `  $ fj issue reopen 42`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
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

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid issue number: %s", opts.Number)
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
