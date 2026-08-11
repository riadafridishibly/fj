package comment

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type replyOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	CommentID  string
	ReviewID   int64
	Body       string
	BodyFile   string
	JSONOutput bool
}

func NewCmdReply(f *cmdutil.Factory) *cobra.Command {
	opts := &replyOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "reply <pr> <comment-id>",
		Short: "Reply to an inline review comment",
		Long: `Reply to an inline review comment on a pull request.

The Forgejo API has no first-class reply threading, so this command posts
a new comment to the same review, file, and line as the target comment.
The web UI groups comments on the same line into one conversation, which
renders as a threaded reply.

Comment ids come from 'fj pr review comment list <pr>'. If you already
know which review holds the comment, pass --review-id to skip the scan.`,
		Example: `  # Reply to comment 4081 on pr #70
  $ fj pr review comment reply 70 4081 --body "Fixed in the latest push"

  # Read the reply body from a file (use "-" for stdin)
  $ fj pr review comment reply 70 4081 --body-file reply.md

  # Machine-readable output (includes the new comment id and review_id)
  $ fj pr review comment reply 70 4081 --body "Done" --json`,
		Args: cmdutil.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			opts.CommentID = args[1]
			return replyRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Reply body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")
	cmd.Flags().Int64Var(&opts.ReviewID, "review-id", 0, "Known review id (skips the scan)")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)
	return cmd
}

func replyRun(opts *replyOptions) error {
	if opts.BodyFile != "" {
		body, err := cmdutil.ReadBodyFromFile(opts.BodyFile)
		if err != nil {
			return err
		}
		opts.Body = body
	}
	if opts.Body == "" {
		return cmdutil.FlagErrorf("--body or --body-file is required")
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

	apiClient, err := opts.Factory.APIClient(repo)
	if err != nil {
		return err
	}

	created, err := apiClient.CreatePullReviewComment(repo.Owner, repo.Name, index, reviewID, forgejo.CreatePullReviewComment{
		Path:       target.Path,
		Body:       opts.Body,
		NewLineNum: int64(target.LineNum),
		OldLineNum: int64(target.OldLineNum),
	})
	if err != nil {
		return fmt.Errorf("replying to comment #%d: %w", commentID, err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, flatComment{PullReviewComment: created, ReviewID: reviewID})
	}

	fmt.Fprintf(os.Stderr, "✓ Replied to comment #%d (new comment #%d)\n", commentID, created.ID)
	if created.HTMLURL != "" {
		fmt.Fprintf(os.Stdout, "%s\n", created.HTMLURL)
	}
	return nil
}
