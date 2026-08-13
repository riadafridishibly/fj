package issue

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type editOptions struct {
	Factory         *cmdutil.Factory
	Args            []string
	Title           string
	Body            string
	BodyFile        string
	AddLabels       []string
	RemoveLabels    []string
	AddAssignees    []string
	RemoveAssignees []string
	Milestone       string
	JSONOutput      bool
}

func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	opts := &editOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "edit <number>",
		Short: "Edit an issue",
		Example: `  $ fj issue edit 42 --title "New title"
  $ fj issue edit 42 --body "Updated description"
  $ fj issue edit 42 --add-label bug --remove-label wontfix
  $ fj issue edit 42 --add-assignee riad
  $ fj issue edit 42 --milestone "v1.0"
  $ fj issue edit 42 --json`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			if opts.BodyFile != "" {
				body, err := cmdutil.ReadBodyFromFile(opts.BodyFile)
				if err != nil {
					return err
				}
				opts.Body = body
			}
			return editRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Edit the title")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Edit the body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmd.Flags().StringSliceVar(&opts.AddLabels, "add-label", nil, "Add labels by name")
	cmd.Flags().StringSliceVar(&opts.RemoveLabels, "remove-label", nil, "Remove labels by name")
	cmd.Flags().StringSliceVar(&opts.AddAssignees, "add-assignee", nil, "Add assignees by username")
	cmd.Flags().StringSliceVar(&opts.RemoveAssignees, "remove-assignee", nil, "Remove assignees by username")
	cmd.Flags().StringVarP(&opts.Milestone, "milestone", "m", "", "Set milestone by name")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func editRun(opts *editOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, _, err := opts.Factory.IssueNumber(repo, opts.Args)
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	editOpt := forgejo.EditIssueOption{}

	if opts.Title != "" {
		editOpt.Title = opts.Title
	}
	if opts.Body != "" {
		editOpt.Body = &opts.Body
	}

	if opts.Milestone != "" {
		msID, err := cmdutil.ResolveMilestoneID(client, repo.Owner, repo.Name, opts.Milestone)
		if err != nil {
			return err
		}
		editOpt.Milestone = &msID
	}

	// Handle assignee changes
	if len(opts.AddAssignees) > 0 || len(opts.RemoveAssignees) > 0 {
		issue, _, err := client.GetIssue(repo.Owner, repo.Name, index)
		if err != nil {
			return fmt.Errorf("getting issue: %w", err)
		}

		assigneeSet := make(map[string]bool)
		for _, a := range issue.Assignees {
			assigneeSet[a.UserName] = true
		}
		for _, a := range opts.AddAssignees {
			assigneeSet[a] = true
		}
		for _, a := range opts.RemoveAssignees {
			delete(assigneeSet, a)
		}

		var assignees []string
		for a := range assigneeSet {
			assignees = append(assignees, a)
		}
		editOpt.Assignees = assignees
	}

	issue, _, err := client.EditIssue(repo.Owner, repo.Name, index, editOpt)
	if err != nil {
		return fmt.Errorf("editing issue: %w", err)
	}

	// Handle label changes separately via label APIs
	if len(opts.AddLabels) > 0 {
		labelIDs, err := cmdutil.ResolveLabelIDs(client, repo.Owner, repo.Name, opts.AddLabels)
		if err != nil {
			return err
		}
		_, _, err = client.AddIssueLabels(repo.Owner, repo.Name, index, forgejo.IssueLabelsOption{
			Labels: labelIDs,
		})
		if err != nil {
			return fmt.Errorf("adding labels: %w", err)
		}
	}

	if len(opts.RemoveLabels) > 0 {
		removeLabelIDs, err := cmdutil.ResolveLabelIDs(client, repo.Owner, repo.Name, opts.RemoveLabels)
		if err != nil {
			return err
		}
		for _, id := range removeLabelIDs {
			_, err := client.DeleteIssueLabel(repo.Owner, repo.Name, index, id)
			if err != nil {
				return fmt.Errorf("removing label: %w", err)
			}
		}
	}

	if opts.JSONOutput {
		// Re-fetch to get updated labels
		issue, _, err = client.GetIssue(repo.Owner, repo.Name, index)
		if err != nil {
			return fmt.Errorf("getting updated issue: %w", err)
		}
		return output.PrintJSON(os.Stdout, issue)
	}

	fmt.Fprintf(os.Stderr, "✓ Edited issue #%d\n", issue.Index)
	fmt.Fprintf(os.Stdout, "%s\n", issue.HTMLURL)
	return nil
}
