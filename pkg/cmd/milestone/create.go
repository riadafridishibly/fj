package milestone

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
	Title       string
	Description string
	DueDate     string
	JSONOutput  bool
}

func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a milestone",
		Example: `  $ fj milestone create --title v1.0
  $ fj milestone create --title v1.0 --description "First stable release"
  $ fj milestone create --title v1.0 --due-date 2026-12-31`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Title == "" {
				return cmdutil.FlagErrorf("--title is required")
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Milestone title")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "Milestone description")
	cmd.Flags().StringVar(&opts.DueDate, "due-date", "", "Due date (YYYY-MM-DD or RFC3339)")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func createRun(opts *createOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	createOpt := forgejo.CreateMilestoneOption{
		Title:       opts.Title,
		Description: opts.Description,
	}
	if opts.DueDate != "" {
		due, err := parseDueDate(opts.DueDate)
		if err != nil {
			return err
		}
		createOpt.Deadline = &due
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	ms, _, err := client.CreateMilestone(repo.Owner, repo.Name, createOpt)
	if err != nil {
		return fmt.Errorf("creating milestone: %w", err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, ms)
	}

	fmt.Fprintf(os.Stderr, "✓ Created milestone %q\n", ms.Title)
	fmt.Fprintln(os.Stdout, webURL(repo, ms.ID))
	return nil
}
