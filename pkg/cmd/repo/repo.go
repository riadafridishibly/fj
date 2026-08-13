package repo

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdRepo(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <command>",
		Short: "Manage repositories",
		Long:  "Work with Forgejo repositories.",
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdStatus(f))
	cmd.AddCommand(NewCmdClone(f))
	cmd.AddCommand(NewCmdFork(f))
	cmd.AddCommand(NewCmdDelete(f))

	return cmd
}
