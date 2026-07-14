// Package comment implements the `fj pr review comment` subcommands for
// managing inline review comments on a pull request.
package comment

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <command>",
		Short: "Manage inline review comments on a pull request",
	}
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdReply(f))
	cmd.AddCommand(NewCmdDelete(f))
	return cmd
}
