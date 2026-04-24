package pr

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type diffOptions struct {
	Factory *cmdutil.Factory
	Number  string
}

func NewCmdDiff(f *cmdutil.Factory) *cobra.Command {
	opts := &diffOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "diff <number>",
		Short: "View the diff of a pull request",
		Example: `  $ fj pr diff 42
  $ fj pr diff 42 | less`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			return diffRun(opts)
		},
	}

	return cmd
}

func diffRun(opts *diffOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid pull request number: %s", opts.Number)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	diff, _, err := client.GetPullRequestDiff(repo.Owner, repo.Name, index, forgejo.PullRequestDiffOptions{})
	if err != nil {
		return fmt.Errorf("getting diff: %w", err)
	}

	_, err = os.Stdout.Write(diff)
	return err
}
