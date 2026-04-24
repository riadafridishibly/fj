package pr

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type closeOptions struct {
	Factory *cmdutil.Factory
	Number  string
	Comment string
}

func NewCmdClose(f *cmdutil.Factory) *cobra.Command {
	opts := &closeOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "close <number>",
		Short: "Close a pull request",
		Example: `  $ fj pr close 42
  $ fj pr close 42 --comment "Closing this PR"`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			return closeRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Comment, "comment", "c", "", "Add a comment before closing")

	return cmd
}

func closeRun(opts *closeOptions) error {
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

	if opts.Comment != "" {
		_, _, err := client.CreateIssueComment(repo.Owner, repo.Name, index, forgejo.CreateIssueCommentOption{
			Body: opts.Comment,
		})
		if err != nil {
			return fmt.Errorf("adding comment: %w", err)
		}
	}

	closed := forgejo.StateClosed
	_, _, err = client.EditPullRequest(repo.Owner, repo.Name, index, forgejo.EditPullRequestOption{
		State: &closed,
	})
	if err != nil {
		return fmt.Errorf("closing pull request: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Closed pull request #%d\n", index)
	return nil
}
