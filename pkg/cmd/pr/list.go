package pr

import (
	"encoding/json"
	"fmt"
	"io"
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
	Milestone  string
	Head       string
	Author     string
	Assignee   string
	Search     string
	Sort       string
	Draft      *bool
	JSONOutput cmdutil.JSONFlags
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}
	var draft bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List pull requests in a repository",
		Aliases: []string{"ls"},
		Example: `  $ fj pr list
  $ fj pr list --state closed
  $ fj pr list --label bug --label urgent
  $ fj pr list --author riad
  $ fj pr list --head feature-1
  $ fj pr list --sort most-commented
  $ fj pr list --draft
  $ fj pr list --json number,title,headRefName
  $ fj pr list --json number,isDraft --jq '.[] | select(.isDraft) | .number'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("draft") {
				opts.Draft = &draft
			}
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of pull requests to list")
	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: open, closed, all")
	cmd.Flags().StringSliceVarP(&opts.Labels, "label", "l", nil, "Filter by label (repeatable; matches PRs with all given labels)")
	cmd.Flags().StringVarP(&opts.Milestone, "milestone", "m", "", "Filter by milestone")
	cmd.Flags().StringVarP(&opts.Head, "head", "H", "", "Filter by head branch")
	cmd.Flags().StringVarP(&opts.Author, "author", "A", "", "Filter by author")
	cmd.Flags().StringVarP(&opts.Assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&opts.Search, "search", "S", "", "Filter by title/body text")
	cmd.Flags().BoolVarP(&draft, "draft", "d", false, "Filter by draft state")
	cmd.Flags().StringVar(&opts.Sort, "sort", "", "Sort order: newest, oldest, recently-updated, most-commented, ...")
	cmdutil.AddJSONFlags(cmd, &opts.JSONOutput, prFields, nil, true)

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

	sortKey, err := cmdutil.SortValue(opts.Sort)
	if err != nil {
		return err
	}

	// The pulls API only filters by state/milestone/sort server-side, so head,
	// author, assignee, label, search and draft are applied client-side. When any of
	// those is active we page through full pages to reduce round trips.
	clientFilter := opts.Head != "" || opts.Author != "" || opts.Assignee != "" ||
		len(opts.Labels) > 0 || opts.Search != "" || opts.Draft != nil
	pageSize := min(opts.Limit, 50)
	if clientFilter {
		pageSize = 50
	}

	listOpt := forgejo.ListPullRequestsOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: pageSize},
		State:       forgejo.StateType(opts.State),
		Sort:        sortKey,
	}

	if opts.Milestone != "" {
		msID, err := cmdutil.ResolveMilestoneID(client, repo.Owner, repo.Name, opts.Milestone)
		if err != nil {
			return err
		}
		listOpt.Milestone = msID
	}

	var allPRs []*apiPullRequest
	var totalCount int
	page := 1
	for len(allPRs) < opts.Limit {
		listOpt.Page = page
		prs, total, err := fetchPRs(opts.Factory, repo, listOpt)
		if err != nil {
			return err
		}
		if page == 1 {
			totalCount = total
		}
		if len(prs) == 0 {
			break
		}
		for _, pr := range prs {
			if !opts.matches(pr) {
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

	if opts.JSONOutput.Enabled() {
		data := make([]map[string]any, len(allPRs))
		for i, pr := range allPRs {
			// ponytail: comments, commits, files and the review fields cost one
			// request per PR each, up to --limit requests per field. Forgejo has
			// no batch endpoint for them.
			if data[i], err = prJSON(opts.Factory, repo, pr, &opts.JSONOutput); err != nil {
				return err
			}
		}
		return opts.JSONOutput.Write(os.Stdout, data)
	}

	if len(allPRs) == 0 {
		fmt.Fprintf(os.Stderr, "No pull requests match your search in %s\n", repo.FullName())
		return nil
	}

	// Status line. The API's X-Total-Count is the unfiltered total, so when we
	// filter client-side we can't quote it as the "of N" total.
	switch {
	case clientFilter:
		fmt.Fprintf(os.Stdout, "\nShowing %d %s pull requests in %s\n\n",
			len(allPRs), opts.State, repo.FullName())
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

// matches reports whether pr passes the client-side filters (head, author,
// assignee, label, search, draft). Comparisons are case-insensitive.
func (opts *listOptions) matches(pr *apiPullRequest) bool {
	if opts.Head != "" && (pr.Head == nil || !strings.EqualFold(pr.Head.Ref, opts.Head)) {
		return false
	}
	if opts.Author != "" && (pr.Poster == nil || !strings.EqualFold(pr.Poster.UserName, opts.Author)) {
		return false
	}
	if opts.Assignee != "" && !hasAssignee(&pr.PullRequest, opts.Assignee) {
		return false
	}
	for _, label := range opts.Labels {
		if !hasLabel(&pr.PullRequest, label) {
			return false
		}
	}
	if opts.Search != "" {
		needle := strings.ToLower(opts.Search)
		if !strings.Contains(strings.ToLower(pr.Title), needle) &&
			!strings.Contains(strings.ToLower(pr.Body), needle) {
			return false
		}
	}
	if opts.Draft != nil && pr.Draft != *opts.Draft {
		return false
	}
	return true
}

func hasAssignee(pr *forgejo.PullRequest, name string) bool {
	if pr.Assignee != nil && strings.EqualFold(pr.Assignee.UserName, name) {
		return true
	}
	for _, a := range pr.Assignees {
		if a != nil && strings.EqualFold(a.UserName, name) {
			return true
		}
	}
	return false
}

func hasLabel(pr *forgejo.PullRequest, name string) bool {
	for _, l := range pr.Labels {
		if l != nil && strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}

// fetchPRs fetches one page of pull requests via the raw REST endpoint, since
// the SDK drops draft, the diff stats and the requested reviewers. It returns
// the page and the unfiltered X-Total-Count header.
func fetchPRs(f *cmdutil.Factory, repo cmdutil.Repo, opt forgejo.ListPullRequestsOptions) ([]*apiPullRequest, int, error) {
	resp, err := f.APIGet(repo, repo.APIPath("/pulls?%s", opt.QueryEncode()))
	if err != nil {
		return nil, 0, fmt.Errorf("listing pull requests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, 0, fmt.Errorf("listing pull requests: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var prs []*apiPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, 0, fmt.Errorf("decoding pull requests: %w", err)
	}
	return prs, cmdutil.TotalCount(&forgejo.Response{Response: resp}), nil
}
