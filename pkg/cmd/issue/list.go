package issue

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Limit      int
	State      string
	Labels     []string
	Assignee   string
	Author     string
	Milestone  string
	Search     string
	JSONOutput bool
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List issues in a repository",
		Aliases: []string{"ls"},
		Example: `  $ fj issue list
  $ fj issue list --state closed
  $ fj issue list --label bug --label urgent
  $ fj issue list --assignee riad
  $ fj issue list --limit 50
  $ fj issue list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of issues to list")
	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: open, closed, all")
	cmd.Flags().StringSliceVarP(&opts.Labels, "label", "l", nil, "Filter by label")
	cmd.Flags().StringVarP(&opts.Assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&opts.Author, "author", "A", "", "Filter by author")
	cmd.Flags().StringVarP(&opts.Milestone, "milestone", "m", "", "Filter by milestone")
	cmd.Flags().StringVarP(&opts.Search, "search", "S", "", "Search issues")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func listRun(opts *listOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	pageSize := min(opts.Limit, 50)

	listOpt := forgejo.ListIssueOption{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: pageSize},
		State:       forgejo.StateType(opts.State),
		Type:        forgejo.IssueTypeIssue,
		Labels:      opts.Labels,
		KeyWord:     opts.Search,
		CreatedBy:   opts.Author,
		AssignedBy:  opts.Assignee,
	}
	if opts.Milestone != "" {
		listOpt.Milestones = []string{opts.Milestone}
	}

	var allIssues []*forgejo.Issue
	var totalCount int
	page := 1
	for len(allIssues) < opts.Limit {
		listOpt.Page = page
		issues, resp, err := client.ListRepoIssues(repo.Owner, repo.Name, listOpt)
		if err != nil {
			return fmt.Errorf("listing issues: %w", err)
		}
		if page == 1 {
			totalCount = cmdutil.TotalCount(resp)
		}
		if len(issues) == 0 {
			break
		}
		allIssues = append(allIssues, issues...)
		if len(issues) < pageSize {
			break
		}
		page++
	}
	if len(allIssues) > opts.Limit {
		allIssues = allIssues[:opts.Limit]
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allIssues)
	}

	if len(allIssues) == 0 {
		fmt.Fprintf(os.Stderr, "No issues match your search in %s\n", repo.FullName())
		return nil
	}

	// Status line
	if totalCount > 0 {
		fmt.Fprintf(os.Stdout, "\nShowing %d of %d %s issues in %s\n\n",
			len(allIssues), totalCount, opts.State, repo.FullName())
	} else {
		fmt.Fprintf(os.Stdout, "\nShowing %d %s issues in %s\n\n",
			len(allIssues), opts.State, repo.FullName())
	}

	// Table with headers. TITLE and LABELS flex to fit the terminal width.
	t := output.NewTable("NUMBER", "TITLE", "LABELS", "UPDATED").Flexible(1, 2)
	for _, issue := range allIssues {
		var labels strings.Builder
		for i, l := range issue.Labels {
			if i > 0 {
				labels.WriteString(", ")
			}
			labels.WriteString(l.Name)
		}

		number := fmt.Sprintf("#%d", issue.Index)
		stateColor := output.Green
		if issue.State == forgejo.StateClosed {
			stateColor = output.Red
		}

		t.AddRow(
			output.Colorize(stateColor, number),
			output.Sanitize(issue.Title),
			output.Colorize(output.Cyan, labels.String()),
			output.Colorize(output.Gray, output.RelativeTimeStr(issue.Updated)),
		)
	}
	t.Render(os.Stdout)
	return nil
}
