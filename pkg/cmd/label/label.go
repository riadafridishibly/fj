package label

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// labelFields are gh's label fields that Forgejo can fill. Left out:
// createdAt, updatedAt and isDefault, which Forgejo lacks, and url, which is
// an API URL in Forgejo, not gh's web page.
var labelFields = []string{"color", "description", "id", "name"}

func NewCmdLabel(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label <command>",
		Short: "Manage labels",
		Long:  "Work with Forgejo repository labels.",
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdEdit(f))
	cmd.AddCommand(NewCmdDelete(f))

	return cmd
}
