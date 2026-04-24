package review

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type createOptions struct {
	Factory *cmdutil.Factory
	Number  string

	Approve        bool
	RequestChanges bool
	Comment        bool

	Body     string
	BodyFile string
	CommitID string

	CommentPath    string
	CommentBody    string
	CommentLine    int64
	CommentOldLine int64

	CommentsFile string

	JSONOutput bool
}

func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "create <number>",
		Short: "Create a review on a pull request",
		Long: `Create a review on a pull request.

Inline comments can be supplied either individually with the --comment-*
flags, or in bulk via --comments-file pointing at a JSON array:

  [
    {
      "path": "path/to/file.go",
      "body": "Comment body",
      "new_position": 42,
      "old_position": 0
    },
    {
      "path": "other/file.go",
      "body": "Another comment",
      "old_position": 10,
      "new_position": 0
    }
  ]

Use new_position for a line in the new file, old_position for a line in
the old file. Set the unused side to 0 (or omit it).`,
		Example: `  $ fj pr review create 42 --approve --body "LGTM"
  $ fj pr review create 42 --request-changes --body-file review.md
  $ fj pr review create 42 --comment --comment-path main.go --comment-line 42 --comment-body "rename this"
  $ fj pr review create 42 --approve --comments-file inline.json
  $ fj pr review create 42 --comment --commit abc123 --body "initial pass"`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			return createRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Approve, "approve", false, "Approve the pull request")
	cmd.Flags().BoolVar(&opts.RequestChanges, "request-changes", false, "Request changes on the pull request")
	cmd.Flags().BoolVar(&opts.Comment, "comment", false, "Submit a comment-only review")
	cmd.MarkFlagsMutuallyExclusive("approve", "request-changes", "comment")

	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Overall review body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmd.Flags().StringVar(&opts.CommitID, "commit", "", "Commit SHA the review applies to")

	cmd.Flags().StringVar(&opts.CommentPath, "comment-path", "", "Path for a single inline comment")
	cmd.Flags().StringVar(&opts.CommentBody, "comment-body", "", "Body for a single inline comment")
	cmd.Flags().Int64Var(&opts.CommentLine, "comment-line", 0, "Line in the new file for a single inline comment")
	cmd.Flags().Int64Var(&opts.CommentOldLine, "comment-old-line", 0, "Line in the old file for a single inline comment")
	cmd.Flags().StringVar(&opts.CommentsFile, "comments-file", "", "JSON file with an array of inline comments")
	cmd.MarkFlagsMutuallyExclusive("comment-path", "comments-file")
	cmd.MarkFlagsMutuallyExclusive("comment-line", "comment-old-line")

	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func createRun(opts *createOptions) error {
	state, err := resolveState(opts)
	if err != nil {
		return err
	}

	if opts.BodyFile != "" {
		body, err := cmdutil.ReadBodyFromFile(opts.BodyFile)
		if err != nil {
			return err
		}
		opts.Body = body
	}

	comments, err := resolveInlineComments(opts)
	if err != nil {
		return err
	}

	if opts.Body == "" && len(comments) == 0 && state == forgejo.ReviewStateComment {
		return cmdutil.FlagErrorf("a comment review requires --body, --body-file, --comment-path, or --comments-file")
	}

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

	review, _, err := client.CreatePullReview(repo.Owner, repo.Name, index, forgejo.CreatePullReviewOptions{
		State:    state,
		Body:     opts.Body,
		CommitID: opts.CommitID,
		Comments: comments,
	})
	if err != nil {
		return fmt.Errorf("creating review: %w", err)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(review)
	}

	fmt.Fprintf(os.Stdout, "%s\n", review.HTMLURL)
	return nil
}

func resolveState(opts *createOptions) (forgejo.ReviewStateType, error) {
	switch {
	case opts.Approve:
		return forgejo.ReviewStateApproved, nil
	case opts.RequestChanges:
		return forgejo.ReviewStateRequestChanges, nil
	case opts.Comment:
		return forgejo.ReviewStateComment, nil
	}
	return "", cmdutil.FlagErrorf("one of --approve, --request-changes, or --comment is required")
}

func resolveInlineComments(opts *createOptions) ([]forgejo.CreatePullReviewComment, error) {
	if opts.CommentsFile != "" {
		data, err := os.ReadFile(opts.CommentsFile)
		if err != nil {
			return nil, fmt.Errorf("reading comments file: %w", err)
		}
		var comments []forgejo.CreatePullReviewComment
		if err := json.Unmarshal(data, &comments); err != nil {
			return nil, fmt.Errorf("parsing comments file: %w", err)
		}
		for i, c := range comments {
			if c.Path == "" || c.Body == "" {
				return nil, cmdutil.FlagErrorf("comments-file entry %d: path and body are required", i)
			}
		}
		return comments, nil
	}

	if opts.CommentPath == "" && opts.CommentBody == "" && opts.CommentLine == 0 && opts.CommentOldLine == 0 {
		return nil, nil
	}
	if opts.CommentPath == "" {
		return nil, cmdutil.FlagErrorf("--comment-path is required with other --comment-* flags")
	}
	if opts.CommentBody == "" {
		return nil, cmdutil.FlagErrorf("--comment-body is required with other --comment-* flags")
	}
	return []forgejo.CreatePullReviewComment{{
		Path:       opts.CommentPath,
		Body:       opts.CommentBody,
		NewLineNum: opts.CommentLine,
		OldLineNum: opts.CommentOldLine,
	}}, nil
}
