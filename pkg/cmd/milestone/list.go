package milestone

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Limit      int
	State      string
	Query      string
	JSONOutput bool
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List milestones in a repository",
		Aliases: []string{"ls"},
		Example: `  $ fj milestone list
  $ fj milestone list --state closed
  $ fj milestone list --query v1
  $ fj milestone list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of milestones to list")
	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: open, closed, all")
	cmd.Flags().StringVarP(&opts.Query, "query", "q", "", "Filter by milestone name")
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

	var allMilestones []*forgejo.Milestone
	var totalCount int
	page := 1
	for len(allMilestones) < opts.Limit {
		milestones, resp, err := client.ListRepoMilestones(repo.Owner, repo.Name, forgejo.ListMilestoneOption{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: pageSize},
			State:       forgejo.StateType(opts.State),
			Name:        opts.Query,
		})
		if err != nil {
			return fmt.Errorf("listing milestones: %w", err)
		}
		if page == 1 {
			totalCount = cmdutil.TotalCount(resp)
		}
		if len(milestones) == 0 {
			break
		}
		allMilestones = append(allMilestones, milestones...)
		if len(milestones) < pageSize {
			break
		}
		page++
	}
	if len(allMilestones) > opts.Limit {
		allMilestones = allMilestones[:opts.Limit]
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allMilestones)
	}

	if len(allMilestones) == 0 {
		fmt.Fprintf(os.Stderr, "No milestones match your search in %s\n", repo.FullName())
		return nil
	}

	if totalCount > 0 {
		fmt.Fprintf(os.Stdout, "\nShowing %d of %d %s milestones in %s\n\n",
			len(allMilestones), totalCount, opts.State, repo.FullName())
	} else {
		fmt.Fprintf(os.Stdout, "\nShowing %d %s milestones in %s\n\n",
			len(allMilestones), opts.State, repo.FullName())
	}

	t := output.NewTable("TITLE", "STATE", "DUE DATE", "OPEN", "CLOSED", "DESCRIPTION").Flexible(0, 5)
	for _, m := range allMilestones {
		stateColor := output.Green
		if m.State == forgejo.StateClosed {
			stateColor = output.Red
		}

		t.AddRow(
			output.Colorize(output.Cyan, output.Sanitize(m.Title)),
			output.Colorize(stateColor, string(m.State)),
			dueDateStr(m),
			strconv.Itoa(m.OpenIssues),
			strconv.Itoa(m.ClosedIssues),
			output.Sanitize(m.Description),
		)
	}
	t.Render(os.Stdout)
	return nil
}
