package pr

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

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
  $ fj pr list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of pull requests to list")
	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: open, closed, all")
	cmd.Flags().StringVarP(&opts.Label, "label", "l", "", "Filter by label")
	cmd.Flags().StringVarP(&opts.Milestone, "milestone", "m", "", "Filter by milestone")
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
	page := 1
	for len(allPRs) < opts.Limit {
		listOpt.Page = page
		prs, _, err := client.ListRepoPullRequests(repo.Owner, repo.Name, listOpt)
		if err != nil {
			return fmt.Errorf("listing pull requests: %w", err)
		}
		if len(prs) == 0 {
			break
		}
		allPRs = append(allPRs, prs...)
		if len(prs) < pageSize {
			break
		}
		page++
	}
	if len(allPRs) > opts.Limit {
		allPRs = allPRs[:opts.Limit]
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, pr := range allPRs {
		status := string(pr.State)
		if pr.HasMerged {
			status = "merged"
		}
		head := ""
		if pr.Head != nil {
			head = pr.Head.Ref
		}
		fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s\n",
			pr.Index,
			output.Truncate(pr.Title, 50),
			head,
			status,
			output.RelativeTimeStr(*pr.Created),
		)
	}
	return w.Flush()
}
