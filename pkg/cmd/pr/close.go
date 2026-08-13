package pr

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
		// gh spells close's argument with braces though it is optional
		// there too; matched verbatim so the usage lines agree.
		Use:   "close {<number> | <url> | <branch>}",
		Short: "Close a pull request",
		Long:  "Close a pull request.\n\n" + cmdutil.PRRefHelp,
		Example: `  $ fj pr close
  $ fj pr close 42
  $ fj pr close 42 --comment "Closing this PR"`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return closeRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Comment, "comment", "c", "", "Add a comment before closing")

	return cmd
}

func closeRun(opts *closeOptions) error {
	repo, index, err := opts.Factory.PRNumber(opts.Args)
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
	_, _, err = client.EditPullRequest(repo.Owner, repo.Name, index, forgejo.EditPullRequestOption{
		State: &closed,
	})
	if err != nil {
		return fmt.Errorf("closing pull request: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Closed pull request #%d\n", index)
	return nil
}
