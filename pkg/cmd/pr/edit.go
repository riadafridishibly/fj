package pr

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type editOptions struct {
	Factory         *cmdutil.Factory
	Number          string
	Title           string
	Body            string
	BodyFile        string
	Base            string
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
		Short: "Edit a pull request",
		Example: `  $ fj pr edit 42 --title "New title"
  $ fj pr edit 42 --body "Updated description"
  $ fj pr edit 42 --add-label bug --remove-label wontfix
  $ fj pr edit 42 --add-assignee riad
  $ fj pr edit 42 --base develop`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
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
	cmd.Flags().StringVarP(&opts.Base, "base", "B", "", "Change base branch")
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

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid pull request number: %s", opts.Number)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	editOpt := forgejo.EditPullRequestOption{}

	if opts.Title != "" {
		editOpt.Title = opts.Title
	}
	if opts.Body != "" {
		editOpt.Body = &opts.Body
	}
	if opts.Base != "" {
		editOpt.Base = opts.Base
	}

	if opts.Milestone != "" {
		msID, err := cmdutil.ResolveMilestoneID(client, repo.Owner, repo.Name, opts.Milestone)
		if err != nil {
			return err
		}
		editOpt.Milestone = msID
	}

	// Handle label changes
	if len(opts.AddLabels) > 0 || len(opts.RemoveLabels) > 0 {
		currentPR, _, err := client.GetPullRequest(repo.Owner, repo.Name, index)
		if err != nil {
			return fmt.Errorf("getting pull request: %w", err)
		}

		// Build current label IDs set
		labelSet := make(map[int64]bool)
		for _, l := range currentPR.Labels {
			labelSet[l.ID] = true
		}

		if len(opts.AddLabels) > 0 {
			addIDs, err := cmdutil.ResolveLabelIDs(client, repo.Owner, repo.Name, opts.AddLabels)
			if err != nil {
				return err
			}
			for _, id := range addIDs {
				labelSet[id] = true
			}
		}

		if len(opts.RemoveLabels) > 0 {
			removeIDs, err := cmdutil.ResolveLabelIDs(client, repo.Owner, repo.Name, opts.RemoveLabels)
			if err != nil {
				return err
			}
			for _, id := range removeIDs {
				delete(labelSet, id)
			}
		}

		var labelIDs []int64
		for id := range labelSet {
			labelIDs = append(labelIDs, id)
		}
		editOpt.Labels = labelIDs
	}

	// Handle assignee changes
	if len(opts.AddAssignees) > 0 || len(opts.RemoveAssignees) > 0 {
		currentPR, _, err := client.GetPullRequest(repo.Owner, repo.Name, index)
		if err != nil {
			return fmt.Errorf("getting pull request: %w", err)
		}

		assigneeSet := make(map[string]bool)
		for _, a := range currentPR.Assignees {
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

	pr, _, err := client.EditPullRequest(repo.Owner, repo.Name, index, editOpt)
	if err != nil {
		return fmt.Errorf("editing pull request: %w", err)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pr)
	}

	fmt.Fprintf(os.Stderr, "✓ Edited pull request #%d\n", pr.Index)
	fmt.Fprintf(os.Stdout, "%s\n", pr.HTMLURL)
	return nil
}
