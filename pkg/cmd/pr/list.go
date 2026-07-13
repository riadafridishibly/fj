package pr

import (
	"encoding/json"
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Limit      int
	State      string
	Label      string
	Milestone  string
	Head       string
	JSONOutput bool
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List pull requests in a repository",
		Aliases: []string{"ls"},
		Example: `  $ fj pr list
  $ fj pr list --state closed
  $ fj pr list --limit 50
  $ fj pr list --head feature-1
  $ fj pr list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of pull requests to list")
	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: open, closed, all")
	cmd.Flags().StringVarP(&opts.Label, "label", "l", "", "Filter by label")
	cmd.Flags().StringVarP(&opts.Milestone, "milestone", "m", "", "Filter by milestone")
	cmd.Flags().StringVarP(&opts.Head, "head", "H", "", "Filter by head branch")
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

	// When filtering client-side (head), grab full pages to reduce round trips.
	pageSize := min(opts.Limit, 50)
	if opts.Head != "" {
		pageSize = 50
	}

	listOpt := forgejo.ListPullRequestsOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: pageSize},
		State:       forgejo.StateType(opts.State),
	}

	if opts.Milestone != "" {
		msID, err := cmdutil.ResolveMilestoneID(client, repo.Owner, repo.Name, opts.Milestone)
		if err != nil {
			return err
		}
		listOpt.Milestone = msID
	}

	var allPRs []*forgejo.PullRequest
	var totalCount int
	page := 1
	for len(allPRs) < opts.Limit {
		listOpt.Page = page
		prs, resp, err := client.ListRepoPullRequests(repo.Owner, repo.Name, listOpt)
		if err != nil {
			return fmt.Errorf("listing pull requests: %w", err)
		}
		if page == 1 {
			totalCount = cmdutil.TotalCount(resp)
		}
		if len(prs) == 0 {
			break
		}
		for _, pr := range prs {
			if opts.Head != "" && (pr.Head == nil || pr.Head.Ref != opts.Head) {
				continue
			}
			allPRs = append(allPRs, pr)
			if len(allPRs) >= opts.Limit {
				break
			}
		}
		if len(prs) < pageSize {
			break
		}
		page++
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allPRs)
	}

	if len(allPRs) == 0 {
		fmt.Fprintf(os.Stderr, "No pull requests match your search in %s\n", repo.FullName())
		return nil
	}

	// Status line. The API's X-Total-Count is the unfiltered total, so when
	// we're filtering client-side (head) we can't quote it as the "of N" total.
	switch {
	case opts.Head != "":
		fmt.Fprintf(os.Stdout, "\nShowing %d %s pull requests with head %q in %s\n\n",
			len(allPRs), opts.State, opts.Head, repo.FullName())
	case totalCount > 0:
		fmt.Fprintf(os.Stdout, "\nShowing %d of %d %s pull requests in %s\n\n",
			len(allPRs), totalCount, opts.State, repo.FullName())
	default:
		fmt.Fprintf(os.Stdout, "\nShowing %d %s pull requests in %s\n\n",
			len(allPRs), opts.State, repo.FullName())
	}

	// Table with headers. TITLE and BRANCH flex to fit the terminal width.
	t := output.NewTable("NUMBER", "TITLE", "BRANCH", "STATUS", "UPDATED").Flexible(1, 2)
	for _, pr := range allPRs {
		status := string(pr.State)
		statusColor := output.Green
		if pr.HasMerged {
			status = "merged"
			statusColor = output.Magenta
		} else if pr.State == forgejo.StateClosed {
			statusColor = output.Red
		}

		head := ""
		if pr.Head != nil {
			head = pr.Head.Ref
		}

		number := fmt.Sprintf("#%d", pr.Index)
		var updated string
		if pr.Updated != nil {
			updated = output.RelativeTimeStr(*pr.Updated)
		}

		t.AddRow(
			output.Colorize(statusColor, number),
			output.Sanitize(pr.Title),
			output.Colorize(output.Cyan, head),
			output.Colorize(statusColor, status),
			output.Colorize(output.Gray, updated),
		)
	}
	t.Render(os.Stdout)
	return nil
}
