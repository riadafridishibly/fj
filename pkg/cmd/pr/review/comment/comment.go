// Package comment implements the `fj pr review comment` subcommands for
// viewing and listing inline review comments on a pull request.
package comment

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <command>",
		Short: "View inline review comments on a pull request",
	}
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdList(f))
	return cmd
}
