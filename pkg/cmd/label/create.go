package label

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type createOptions struct {
	Factory     *cmdutil.Factory
	Name        string
	Color       string
	Description string
	JSONOutput  bool
}

func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a label",
		Example: `  $ fj label create --name bug --color "#ee0701"
  $ fj label create --name enhancement --color "#a2eeef" --description "New feature"
  $ fj label create --name "help wanted"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Name == "" {
				return cmdutil.FlagErrorf("--name is required")
			}
			if opts.Color == "" {
				opts.Color = cmdutil.RandomLabelColor()
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "Label name")
	cmd.Flags().StringVarP(&opts.Color, "color", "c", "", "Label color (hex, e.g. \"#ee0701\"). Random if omitted")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "Label description")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func createRun(opts *createOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	label, _, err := client.CreateLabel(repo.Owner, repo.Name, forgejo.CreateLabelOption{
		Name:        opts.Name,
		Color:       opts.Color,
		Description: opts.Description,
	})
	if err != nil {
		return fmt.Errorf("creating label: %w", err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, label)
	}

	fmt.Fprintf(os.Stderr, "✓ Created label %q\n", label.Name)
	return nil
}
