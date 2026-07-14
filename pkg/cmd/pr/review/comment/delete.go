package comment

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory   *cmdutil.Factory
	Number    string
	CommentID string
	ReviewID  int64
	Yes       bool
	DryRun    bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <pr> <comment-id>",
		Short: "Delete an inline review comment",
		Long: `Delete an inline review comment on a pull request.

The comment is fetched first and its author, file, and body are shown, so you
can verify the id refers to the comment you intend to delete. Deletion cannot
be undone; --yes is required to proceed, or pass --dry-run to preview.

Comment ids come from 'fj pr review comment list <pr>'. If you already know
which review holds the comment, pass --review-id to skip the scan.`,
		Example: `  # Shows the comment's author, file, and body, then deletes it
  $ fj pr review comment delete 70 4081 --yes

  # Preview the target without deleting
  $ fj pr review comment delete 70 4081 --dry-run`,
		Args: cmdutil.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			opts.CommentID = args[1]
			return deleteRun(opts)
		},
	}

	cmd.Flags().Int64Var(&opts.ReviewID, "review-id", 0, "Known review id (skips the scan)")
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

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid pull request number: %s", opts.Number)
	}

	commentID, err := strconv.ParseInt(opts.CommentID, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid comment id: %s", opts.CommentID)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	target, reviewID, err := findReviewComment(client, repo.Owner, repo.Name, index, commentID, opts.ReviewID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("no inline review comment with id %d on pr #%d", commentID, index)
	}

	author := "unknown"
	if target.Reviewer != nil {
		author = target.Reviewer.UserName
	}
	fmt.Fprintf(os.Stderr, "Comment #%d by %s on %s (review #%d):\n  %s\n",
		target.ID, author, target.Path, reviewID, excerpt(target.Body))

	if !perform {
		fmt.Fprintln(os.Stderr, "(dry-run; no changes were made)")
		return nil
	}

	apiClient, err := opts.Factory.APIClient(repo)
	if err != nil {
		return err
	}

	if err := apiClient.DeletePullReviewComment(repo.Owner, repo.Name, index, reviewID, commentID); err != nil {
		return fmt.Errorf("deleting review comment #%d: %w", commentID, err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted review comment #%d\n", commentID)
	return nil
}

// excerpt renders the first line of a comment body, truncated for display.
func excerpt(body string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	if len(line) > 80 {
		line = line[:77] + "..."
	}
	return line
}
