// Package review implements the `fj pr review` subcommands for creating,
// listing, and viewing pull request reviews.
package review

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/pr/review/comment"
)

func NewCmdReview(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review <command>",
		Short: "Manage pull request reviews",
	}
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(comment.NewCmdComment(f))
	return cmd
}
