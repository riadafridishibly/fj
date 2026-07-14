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
	Yes     bool
	DryRun  bool
}

func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a label",
		Example: `  $ fj label delete bug --yes
  $ fj label delete "help wanted" --dry-run`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return deleteRun(opts)
		},
	}

	cmdutil.AddDeleteFlags(cmd, &opts.Yes, &opts.DryRun)

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	perform, err := cmdutil.ResolveDeleteFlags(opts.Yes, opts.DryRun)
	if err != nil {
		return err
	}

	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Find label by name (case-insensitive).
	labels, _, err := client.ListRepoLabels(repo.Owner, repo.Name, forgejo.ListLabelsOptions{})
	if err != nil {
		return fmt.Errorf("listing labels: %w", err)
	}

	var label *forgejo.Label
	for _, l := range labels {
		if strings.EqualFold(l.Name, opts.Name) {
			label = l
			break
		}
	}
	if label == nil {
		return fmt.Errorf("label not found: %s", opts.Name)
	}

	fmt.Fprintf(os.Stderr, "Label #%d %q (color %s)\n", label.ID, label.Name, label.Color)

	if !perform {
		fmt.Fprintln(os.Stderr, "(dry-run; no changes were made)")
		return nil
	}

	if _, err := client.DeleteLabel(repo.Owner, repo.Name, label.ID); err != nil {
		return fmt.Errorf("deleting label: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted label %q\n", opts.Name)
	return nil
}
