package milestone

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type editOptions struct {
	Factory     *cmdutil.Factory
	Name        string
	Title       string
	Description string
	DueDate     string
	JSONOutput  bool
}

func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	opts := &editOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "edit <milestone>",
		Short: "Edit a milestone",
		Example: `  $ fj milestone edit v1.0 --title v1.1
  $ fj milestone edit v1.0 --description "Updated scope"
  $ fj milestone edit v1.0 --due-date 2027-01-31`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]

			editOpt := forgejo.EditMilestoneOption{Title: opts.Title}
			if cmd.Flags().Changed("description") {
				editOpt.Description = &opts.Description
			}
			if opts.DueDate != "" {
				due, err := parseDueDate(opts.DueDate)
				if err != nil {
					return err
				}
				editOpt.Deadline = &due
			}
			if editOpt.Title == "" && editOpt.Description == nil && editOpt.Deadline == nil {
				return cmdutil.FlagErrorf("specify at least one of --title, --description, or --due-date")
			}
			return editRun(opts, editOpt)
		},
	}

	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Rename the milestone")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "Change milestone description")
	cmd.Flags().StringVar(&opts.DueDate, "due-date", "", "Change due date (YYYY-MM-DD or RFC3339)")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func editRun(opts *editOptions, editOpt forgejo.EditMilestoneOption) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	ms, _, err := client.EditMilestoneByName(repo.Owner, repo.Name, opts.Name, editOpt)
	if err != nil {
		return fmt.Errorf("editing milestone: %w", err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, ms)
	}

	fmt.Fprintf(os.Stderr, "✓ Edited milestone %q\n", ms.Title)
	return nil
}
