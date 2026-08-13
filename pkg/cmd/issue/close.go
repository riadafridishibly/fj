package issue

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type closeOptions struct {
	Factory *cmdutil.Factory
	Args    []string
	Comment string
}

func NewCmdClose(f *cmdutil.Factory) *cobra.Command {
	opts := &closeOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "close " + cmdutil.IssueRefSpec,
		Short: "Close an issue",
		Long:  "Close an issue.\n\n" + cmdutil.IssueRefHelp,
		Example: `  $ fj issue close 42
  $ fj issue close 42 --comment "Closing as duplicate"`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return closeRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Comment, "comment", "c", "", "Add a comment before closing")

	return cmd
}

func closeRun(opts *closeOptions) error {
	repo, index, err := opts.Factory.IssueNumber(opts.Args)
	if err != nil {
		return err
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
	_, _, err = client.EditIssue(repo.Owner, repo.Name, index, forgejo.EditIssueOption{
		State: &closed,
	})
	if err != nil {
		return fmt.Errorf("closing issue: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Closed issue #%d\n", index)
	return nil
}
