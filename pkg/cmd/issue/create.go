package issue

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type createOptions struct {
	Factory    *cmdutil.Factory
	Title      string
	Body       string
	BodyFile   string
	Assignees  []string
	Labels     []string
	Milestone  string
	JSONOutput bool
}

func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new issue",
		Aliases: []string{"new"},
		Example: `  $ fj issue create --title "Bug report" --body "Description of the bug"
  $ fj issue create --title "Feature" --label enhancement --assignee riad
  $ fj issue create --title "Bug" --body-file bug-report.md
  $ fj issue create --title "Bug" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Title == "" {
				return cmdutil.FlagErrorf("--title is required")
			}
			if opts.BodyFile != "" {
				body, err := cmdutil.ReadBodyFromFile(opts.BodyFile)
				if err != nil {
					return err
				}
				opts.Body = body
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Title for the issue")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Body for the issue")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmd.Flags().StringSliceVarP(&opts.Assignees, "assignee", "a", nil, "Assign users by their username")
	cmd.Flags().StringSliceVarP(&opts.Labels, "label", "l", nil, "Add labels by name")
	cmd.Flags().StringVarP(&opts.Milestone, "milestone", "m", "", "Add to a milestone by name")
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

	createOpt := forgejo.CreateIssueOption{
		Title:     opts.Title,
		Body:      opts.Body,
		Assignees: opts.Assignees,
	}

	if len(opts.Labels) > 0 {
		labelIDs, err := cmdutil.ResolveLabelIDs(client, repo.Owner, repo.Name, opts.Labels)
		if err != nil {
			return err
		}
		createOpt.Labels = labelIDs
	}

	if opts.Milestone != "" {
		msID, err := cmdutil.ResolveMilestoneID(client, repo.Owner, repo.Name, opts.Milestone)
		if err != nil {
			return err
		}
		createOpt.Milestone = msID
	}

	issue, _, err := client.CreateIssue(repo.Owner, repo.Name, createOpt)
	if err != nil {
		return fmt.Errorf("creating issue: %w", err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, issue)
	}

	fmt.Fprintf(os.Stdout, "%s\n", issue.HTMLURL)
	return nil
}
