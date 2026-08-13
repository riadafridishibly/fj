// Package review implements the `fj pr review` subcommands for creating,
// listing, and viewing pull request reviews.
package review

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/pr/review/comment"
)

// NewCmdReview returns the `review` parent. gh spells reviewing as a leaf
// verb — `gh pr review 42 --approve` — so the group itself submits a
// review, dispatching to `create`.
func NewCmdReview(f *cmdutil.Factory) *cobra.Command {
	cmd := newCreateCmd(f)
	cmd.Use = "review " + cmdutil.PRRefSpec + " [flags]"
	cmd.Short = "Manage pull request reviews"
	cmd.Long = "Manage pull request reviews.\n\n" +
		"With a review-state flag the command submits a review, exactly as " +
		"fj pr review create does.\n\n" + cmdutil.PRRefHelp
	cmd.Example = `  $ fj pr review 42 --approve --body "LGTM"
  $ fj pr review --request-changes --body "see comments"
  $ fj pr review list 42`

	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(comment.NewCmdComment(f))
	return cmd
}
