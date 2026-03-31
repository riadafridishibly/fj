package label

import (
	"fmt"
	"os"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory *cmdutil.Factory
	Name    string
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a label",
		Example: `  $ fj label delete bug
  $ fj label delete "help wanted"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return deleteRun(opts)
		},
	}

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Find label by name
	labels, _, err := client.ListRepoLabels(repo.Owner, repo.Name, forgejo.ListLabelsOptions{})
	if err != nil {
		return fmt.Errorf("listing labels: %w", err)
	}

	var labelID int64
	for _, l := range labels {
		if strings.EqualFold(l.Name, opts.Name) {
			labelID = l.ID
			break
		}
	}
	if labelID == 0 {
		return fmt.Errorf("label not found: %s", opts.Name)
	}

	_, err = client.DeleteLabel(repo.Owner, repo.Name, labelID)
	if err != nil {
		return fmt.Errorf("deleting label: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted label %q\n", opts.Name)
	return nil
}
