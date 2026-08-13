package review

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	ReviewID   string
	JSONOutput bool
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view <number> <review-id>",
		Short: "View a pull request review and its inline comments",
		Example: `  $ fj pr review view 42 12345
  $ fj pr review view 42 12345 --json`,
		Args: cmdutil.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			opts.ReviewID = args[1]
			return viewRun(opts)
		},
	}

	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)
	return cmd
}

func viewRun(opts *viewOptions) error {
	repo, index, err := opts.Factory.PRNumber([]string{opts.Number})
	if err != nil {
		return err
	}

	reviewID, err := strconv.ParseInt(opts.ReviewID, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid review id: %s", opts.ReviewID)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	review, _, err := client.GetPullReview(repo.Owner, repo.Name, index, reviewID)
	if err != nil {
		return fmt.Errorf("getting review: %w", err)
	}

	comments, _, err := client.ListPullReviewComments(repo.Owner, repo.Name, index, reviewID)
	if err != nil {
		return fmt.Errorf("listing review comments: %w", err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, map[string]any{
			"review":   review,
			"comments": comments,
		})
	}

	reviewer := "unknown"
	if review.Reviewer != nil {
		reviewer = review.Reviewer.UserName
	} else if review.ReviewerTeam != nil {
		reviewer = "@" + review.ReviewerTeam.Name
	}

	fmt.Fprintf(os.Stdout, "Review #%d on PR #%d\n", review.ID, index)
	fmt.Fprintf(os.Stdout, "State: %s\n", output.Colorize(stateColor(review.State), string(review.State)))
	fmt.Fprintf(os.Stdout, "Reviewer: %s\n", reviewer)
	if review.CommitID != "" {
		fmt.Fprintf(os.Stdout, "Commit: %s\n", review.CommitID)
	}
	fmt.Fprintf(os.Stdout, "Submitted: %s\n", output.RelativeTimeStr(review.Submitted))
	if review.Stale {
		fmt.Fprintf(os.Stdout, "Stale: true\n")
	}
	if review.Dismissed {
		fmt.Fprintf(os.Stdout, "Dismissed: true\n")
	}

	if review.Body != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", output.RenderMarkdown(review.Body))
	}

	if len(comments) > 0 {
		fmt.Fprintf(os.Stdout, "\n--- Inline comments (%d) ---\n", len(comments))
		for _, c := range comments {
			line := inlineLine(c)
			fmt.Fprintf(os.Stdout, "\n%s%s:\n%s\n", c.Path, line, output.RenderMarkdown(c.Body))
		}
	}

	if review.HTMLURL != "" {
		fmt.Fprintf(os.Stdout, "\nView on web: %s\n", review.HTMLURL)
	}
	return nil
}

func inlineLine(c *forgejo.PullReviewComment) string {
	switch {
	case c.LineNum != 0:
		return ":" + strconv.FormatUint(c.LineNum, 10)
	case c.OldLineNum != 0:
		return ":-" + strconv.FormatUint(c.OldLineNum, 10)
	}
	return ""
}
