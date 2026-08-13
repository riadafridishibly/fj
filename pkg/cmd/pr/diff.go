package pr

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type diffOptions struct {
	Factory *cmdutil.Factory
	Args    []string
}

func NewCmdDiff(f *cmdutil.Factory) *cobra.Command {
	opts := &diffOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "diff " + cmdutil.PRRefSpec,
		Short: "View the diff of a pull request",
		Long:  "View the diff of a pull request.\n\n" + cmdutil.PRRefHelp,
		Example: `  $ fj pr diff
  $ fj pr diff 42
  $ fj pr diff 42 | less`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return diffRun(opts)
		},
	}

	return cmd
}

func diffRun(opts *diffOptions) error {
	repo, index, err := opts.Factory.PRNumber(opts.Args)
	if err != nil {
		return err
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
