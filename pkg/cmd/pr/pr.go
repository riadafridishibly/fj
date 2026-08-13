package pr

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/pr/review"
)

func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr <command>",
		Short: "Manage pull requests",
		Long:  "Work with Forgejo pull requests.",
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdStatus(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdClose(f))
	cmd.AddCommand(NewCmdEdit(f))
	cmd.AddCommand(NewCmdComment(f))
	cmd.AddCommand(review.NewCmdReview(f))
	cmd.AddCommand(NewCmdMerge(f))
	cmd.AddCommand(NewCmdDiff(f))
	cmd.AddCommand(NewCmdCheckout(f))

	return cmd
}
