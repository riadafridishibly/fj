package issue

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdIssue(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue <command>",
		Short: "Manage issues",
		Long:  "Work with Forgejo issues.",
		Aliases: []string{"i"},
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdClose(f))
	cmd.AddCommand(NewCmdReopen(f))
	cmd.AddCommand(NewCmdEdit(f))
	cmd.AddCommand(NewCmdComment(f))
	cmd.AddCommand(NewCmdDelete(f))

	return cmd
}
