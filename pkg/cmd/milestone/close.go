package milestone

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdClose(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "close <milestone>",
		Short:   "Close a milestone",
		Example: `  $ fj milestone close v1.0`,
		Args:    cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ms, err := setState(f, args[0], forgejo.StateClosed)
			if err != nil {
				return fmt.Errorf("closing milestone: %w", err)
			}
			fmt.Fprintf(os.Stderr, "✓ Closed milestone %q\n", ms.Title)
			return nil
		},
	}

	return cmd
}
