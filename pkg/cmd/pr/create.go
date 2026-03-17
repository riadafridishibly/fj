package pr

import (
	"encoding/json"
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/git"
)

type createOptions struct {
	Factory    *cmdutil.Factory
	Title      string
	Body       string
	BodyFile   string
	Base       string
	Head       string
	Assignees  []string
	Labels     []string
	Milestone  string
	Draft      bool
	JSONOutput bool
}

func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a pull request",
		Aliases: []string{"new"},
		Example: `  $ fj pr create --title "Add feature" --body "Description"
  $ fj pr create --title "Fix" --base main --head feature-branch
  $ fj pr create --title "Fix" --label bug --assignee riad
  $ fj pr create --title "WIP" --draft
  $ fj pr create --title "Fix" --body-file description.md`,
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

	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Title for the pull request")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Body for the pull request")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmd.Flags().StringVarP(&opts.Base, "base", "B", "", "The base branch (default: repo default branch)")
	cmd.Flags().StringVarP(&opts.Head, "head", "H", "", "The head branch (default: current branch)")
	cmd.Flags().StringSliceVarP(&opts.Assignees, "assignee", "a", nil, "Assign users by their username")
	cmd.Flags().StringSliceVarP(&opts.Labels, "label", "l", nil, "Add labels by name")
	cmd.Flags().StringVarP(&opts.Milestone, "milestone", "m", "", "Add to a milestone by name")
	cmd.Flags().BoolVarP(&opts.Draft, "draft", "d", false, "Mark as draft/work-in-progress")
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

	head := opts.Head
	if head == "" {
		head, err = git.CurrentBranch()
		if err != nil {
			return fmt.Errorf("could not determine current branch: %w", err)
		}
	}

	base := opts.Base
	if base == "" {
		// Get repo default branch
		r, _, err := client.GetRepo(repo.Owner, repo.Name)
		if err != nil {
			return fmt.Errorf("getting repository: %w", err)
		}
		base = r.DefaultBranch
	}

	title := opts.Title
	if opts.Draft {
		// Forgejo uses WIP: prefix for draft PRs
		title = "WIP: " + title
	}

	createOpt := forgejo.CreatePullRequestOption{
		Head:      head,
		Base:      base,
		Title:     title,
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

	pr, _, err := client.CreatePullRequest(repo.Owner, repo.Name, createOpt)
	if err != nil {
		return fmt.Errorf("creating pull request: %w", err)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pr)
	}

	fmt.Fprintf(os.Stdout, "%s\n", pr.HTMLURL)
	return nil
}
