package label

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type editOptions struct {
	Factory     *cmdutil.Factory
	Name        string
	NewName     string
	Color       string
	Description string
	JSONOutput  bool
}

func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	opts := &editOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit a label",
		Example: `  $ fj label edit bug --color "#ff0000"
  $ fj label edit bug --new-name bugfix
  $ fj label edit bug --description "Something is broken"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return editRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.NewName, "new-name", "", "Rename the label")
	cmd.Flags().StringVarP(&opts.Color, "color", "c", "", "Change label color (hex)")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "Change label description")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func editRun(opts *editOptions) error {
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

	editOpt := forgejo.EditLabelOption{}
	if opts.NewName != "" {
		editOpt.Name = &opts.NewName
	}
	if opts.Color != "" {
		editOpt.Color = &opts.Color
	}
	if opts.Description != "" {
		editOpt.Description = &opts.Description
	}

	label, _, err := client.EditLabel(repo.Owner, repo.Name, labelID, editOpt)
	if err != nil {
		return fmt.Errorf("editing label: %w", err)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(label)
	}

	fmt.Fprintf(os.Stderr, "✓ Edited label %q\n", label.Name)
	return nil
}
