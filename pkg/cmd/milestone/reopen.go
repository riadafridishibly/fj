package milestone

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdReopen(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reopen <milestone>",
		Short:   "Reopen a milestone",
		Example: `  $ fj milestone reopen v1.0`,
		Args:    cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ms, err := setState(f, args[0], forgejo.StateOpen)
			if err != nil {
				return fmt.Errorf("reopening milestone: %w", err)
			}
			fmt.Fprintf(os.Stderr, "✓ Reopened milestone %q\n", ms.Title)
			return nil
		},
	}

	return cmd
}
