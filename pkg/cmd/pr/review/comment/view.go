package comment

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/debug"
	"github.com/riadafridishibly/fj/internal/output"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	CommentID  string
	ReviewID   int64
	JSONOutput bool
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view <pr> <comment-id>",
		Short: "View an inline review comment by its ID",
		Long: `View an inline review comment on a pull request.

Inline review comments are addressed by review id + comment id in the
Forgejo API, but a URL fragment like #issuecomment-4081 only exposes the
comment id. This command walks reviews on the given PR to find the
matching comment id.

If you already know which review the comment belongs to (e.g. from
'fj pr review list <pr>'), pass --review-id to skip the scan.`,
		Example: `  # Scan every review on pr #70 for comment id 4081
  $ fj pr review comment view 70 4081

  # Skip the scan if you already know the review id
  $ fj pr review comment view 70 4081 --review-id 12345

  # Machine-readable output
  $ fj pr review comment view 70 4081 --json`,
		Args: cmdutil.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			opts.CommentID = args[1]
			return viewRun(opts)
		},
	}

	cmd.Flags().Int64Var(&opts.ReviewID, "review-id", 0, "Known review id (skips the scan)")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)
	return cmd
}

func viewRun(opts *viewOptions) error {
	repo, index, err := opts.Factory.PRNumber([]string{opts.Number})
	if err != nil {
		return err
	}

	commentID, err := strconv.ParseInt(opts.CommentID, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid comment id: %s", opts.CommentID)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	found, reviewID, err := findReviewComment(client, repo.Owner, repo.Name, index, commentID, opts.ReviewID)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("no inline review comment with id %d on pr #%d", commentID, index)
	}

	if opts.JSONOutput {
		payload := map[string]any{
			"comment":   found,
			"review_id": reviewID,
		}
		return output.PrintJSON(os.Stdout, payload)
	}

	author := "unknown"
	if found.Reviewer != nil {
		author = found.Reviewer.UserName
	}
	line := ""
	switch {
	case found.LineNum != 0:
		line = ":" + strconv.FormatUint(found.LineNum, 10)
	case found.OldLineNum != 0:
		line = ":-" + strconv.FormatUint(found.OldLineNum, 10)
	}

	fmt.Fprintf(os.Stdout, "Comment #%d by %s (review #%d)\n", found.ID, author, reviewID)
	fmt.Fprintf(os.Stdout, "File: %s%s\n", found.Path, line)
	fmt.Fprintf(os.Stdout, "Created: %s\n", output.RelativeTimeStr(found.Created))
	if !found.Updated.Equal(found.Created) {
		fmt.Fprintf(os.Stdout, "Updated: %s\n", output.RelativeTimeStr(found.Updated))
	}
	if found.Resolver != nil {
		fmt.Fprintf(os.Stdout, "Resolved by: %s\n", found.Resolver.UserName)
	}
	if found.Body != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", output.RenderMarkdown(found.Body))
	}
	if found.HTMLURL != "" {
		fmt.Fprintf(os.Stdout, "\nView on web: %s\n", found.HTMLURL)
	}
	return nil
}

// findReviewComment locates a comment by id. If reviewHint is non-zero,
// only that review is scanned; otherwise every review on the PR is walked.
func findReviewComment(client *forgejo.Client, owner, repo string, index, commentID, reviewHint int64) (*forgejo.PullReviewComment, int64, error) {
	if reviewHint != 0 {
		comments, err := listReviewComments(client, owner, repo, index, reviewHint)
		if err != nil {
			return nil, 0, err
		}
		for _, c := range comments {
			if c.ID == commentID {
				return c, reviewHint, nil
			}
		}
		return nil, 0, nil
	}

	reviews, err := listAllReviews(client, owner, repo, index)
	if err != nil {
		return nil, 0, err
	}
	debug.Logf(1, "scanning %d reviews for comment id %d", len(reviews), commentID)
	for _, r := range reviews {
		if r.CodeCommentsCount == 0 {
			debug.Logf(3, "skipping review=%d (no inline comments)", r.ID)
			continue
		}
		comments, err := listReviewComments(client, owner, repo, index, r.ID)
		if err != nil {
			return nil, 0, err
		}
		for _, c := range comments {
			if c.ID == commentID {
				return c, r.ID, nil
			}
		}
	}
	return nil, 0, nil
}
