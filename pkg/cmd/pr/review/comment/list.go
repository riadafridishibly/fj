package comment

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	ReviewID   int64
	JSONOutput bool
}

// flatComment is the JSON shape emitted by `list --json`. It attaches
// review_id to each comment so callers can address it back to the
// /pulls/{index}/reviews/{review_id}/comments/{id} endpoint.
type flatComment struct {
	*forgejo.PullReviewComment
	ReviewID int64 `json:"review_id"`
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list <pr>",
		Aliases: []string{"ls"},
		Short:   "List all inline review comments on a pull request",
		Long: `List inline review comments across every review on a pull request.

JSON output attaches a review_id field to each comment so it can be
addressed back to the Forgejo API.

Pass --review-id to limit the output to a single review (you can get
review ids from 'fj pr review list <pr>').

With DEBUG=2 or DEBUG=3 in the environment, each API call is logged and
underlying errors are shown verbatim.`,
		Example: `  # All inline comments across every review on pr #70
  $ fj pr review comment list 70

  # Only the comments from a specific review
  $ fj pr review comment list 70 --review-id 12345

  # Machine-readable output
  $ fj pr review comment list 70 --json`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			return listRun(opts)
		},
	}

	cmd.Flags().Int64Var(&opts.ReviewID, "review-id", 0, "Limit to a single review id")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)
	return cmd
}

func listRun(opts *listOptions) error {
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

	var flat []flatComment

	if opts.ReviewID != 0 {
		comments, err := listReviewComments(client, repo.Owner, repo.Name, index, opts.ReviewID)
		if err != nil {
			return err
		}
		for _, c := range comments {
			flat = append(flat, flatComment{PullReviewComment: c, ReviewID: opts.ReviewID})
		}
	} else {
		reviews, err := listAllReviews(client, repo.Owner, repo.Name, index)
		if err != nil {
			return err
		}
		for _, r := range reviews {
			if r.CodeCommentsCount == 0 {
				continue
			}
			comments, err := listReviewComments(client, repo.Owner, repo.Name, index, r.ID)
			if err != nil {
				return err
			}
			for _, c := range comments {
				flat = append(flat, flatComment{PullReviewComment: c, ReviewID: r.ID})
			}
		}
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(flat)
	}

	if len(flat) == 0 {
		fmt.Fprintf(os.Stderr, "No inline review comments on pr #%d in %s\n", index, repo.FullName())
		return nil
	}

	t := output.NewTable("ID", "REVIEW_ID", "AUTHOR", "PATH", "LINE", "CREATED")
	for _, c := range flat {
		author := ""
		if c.Reviewer != nil {
			author = c.Reviewer.UserName
		}
		line := ""
		switch {
		case c.LineNum != 0:
			line = strconv.FormatUint(c.LineNum, 10)
		case c.OldLineNum != 0:
			line = "-" + strconv.FormatUint(c.OldLineNum, 10)
		}
		t.AddRow(
			strconv.FormatInt(c.ID, 10),
			strconv.FormatInt(c.ReviewID, 10),
			author,
			output.Truncate(c.Path, 40),
			line,
			output.Colorize(output.Gray, output.RelativeTimeStr(c.Created)),
		)
	}
	t.Render(os.Stdout)
	return nil
}
