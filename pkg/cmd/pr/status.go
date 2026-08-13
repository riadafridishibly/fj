package pr

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/debug"
	"github.com/riadafridishibly/fj/internal/git"
	"github.com/riadafridishibly/fj/internal/output"
)

// statusLimit caps how many pull requests a section lists; the rest become
// the "And N more" tail.
const statusLimit = 10

type statusOptions struct {
	Factory    *cmdutil.Factory
	JSONOutput bool
}

func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	opts := &statusOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of relevant pull requests",
		Long: `Show the open pull requests in this repository that concern you: the one for the
current branch, the ones you opened, and the ones waiting on a review from you.

Each pull request is followed by the state of its head commit's checks, when the
commit has any.`,
		Example: `  $ fj pr status
  $ fj pr status --json`,
		Args: cmdutil.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(opts)
		},
	}

	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

// prStatusEntry is one pull request as a status report describes it, and the
// single representation behind both renderings: the text sections are built
// from the same entries the --json payload carries.
type prStatusEntry struct {
	Number  int64  `json:"number"`
	Title   string `json:"title"`
	Branch  string `json:"branch,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
	// Checks is the combined state of the head commit's statuses
	// ("success", "pending", "failure", ...), empty when it has none.
	Checks string `json:"checks,omitempty"`
}

type prStatus struct {
	Repository string `json:"repository"`
	// Branch is the checked-out branch, empty outside a git repository.
	Branch          string          `json:"branch,omitempty"`
	Current         *prStatusEntry  `json:"current_branch,omitempty"`
	Authored        []prStatusEntry `json:"created_by_you"`
	ReviewRequested []prStatusEntry `json:"review_requested"`
}

func statusRun(opts *statusOptions) error {
	defer debug.Track(1, "fj pr status (total)")()

	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}
	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}
	me, _, err := client.GetMyUserInfo()
	if err != nil {
		return fmt.Errorf("getting the authenticated user: %w", err)
	}

	// One listing feeds every section: the current branch's pull request and
	// the ones you opened are both views of the same open set, and it is what
	// gives the review-request search its head branches back.
	open, err := opts.Factory.ListOpenPRs(repo)
	if err != nil {
		return err
	}
	byIndex := make(map[int64]*forgejo.PullRequest, len(open))
	for _, pr := range open {
		byIndex[pr.Index] = pr
	}

	branch, _ := git.CurrentBranch()
	var currentPRs []*forgejo.PullRequest
	if branch != "" {
		current, err := cmdutil.PickBranchPR(open, repo, branch)
		if err != nil {
			return err
		}
		if current != nil {
			currentPRs = []*forgejo.PullRequest{current}
		}
	}

	var authored []*forgejo.PullRequest
	for _, pr := range open {
		if pr.Poster != nil && strings.EqualFold(pr.Poster.UserName, me.UserName) {
			authored = append(authored, pr)
		}
	}

	requested, err := reviewRequested(opts.Factory, repo, byIndex)
	if err != nil {
		return err
	}

	checks := &prChecks{client: client, repo: repo, cache: map[string]string{}}
	current := statusEntries(currentPRs, checks)
	status := &prStatus{
		Repository:      repo.FullName(),
		Branch:          branch,
		Authored:        statusEntries(authored, checks),
		ReviewRequested: statusEntries(requested, checks),
	}
	if len(current) > 0 {
		status.Current = &current[0]
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, status)
	}

	noCurrent := "There is no current branch"
	if branch != "" {
		noCurrent = "There is no pull request associated with " + output.Colorize(output.Cyan, "["+branch+"]")
	}

	w := os.Stdout
	fmt.Fprintf(w, "\n%s\n\n", output.Colorize(output.Bold, "Relevant pull requests in "+status.Repository))
	output.StatusSection(w, "Current branch", noCurrent, sectionLines(current))
	output.StatusSection(w, "Created by you", "You have no open pull requests", sectionLines(status.Authored))
	output.StatusSection(w, "Requesting a code review from you", "You have no pull requests to review",
		sectionLines(status.ReviewRequested))
	return nil
}

// reviewRequested lists the repository's open pull requests waiting on a
// review from the caller. Only the search endpoint answers that question,
// and it returns issue-shaped results spanning every repository, so each hit
// is filtered to this repository and matched back to the full pull request.
func reviewRequested(f *cmdutil.Factory, repo cmdutil.Repo, byIndex map[int64]*forgejo.PullRequest) ([]*forgejo.PullRequest, error) {
	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	found, err := client.SearchIssues(api.SearchIssuesOptions{
		Type:            "pulls",
		State:           "open",
		Owner:           repo.Owner,
		ReviewRequested: true,
	})
	if err != nil {
		return nil, fmt.Errorf("searching for requested reviews: %w", err)
	}

	var prs []*forgejo.PullRequest
	for _, issue := range found {
		if issue.Repository == nil || !strings.EqualFold(issue.Repository.FullName, repo.FullName()) {
			continue
		}
		if pr, ok := byIndex[issue.Index]; ok {
			prs = append(prs, pr)
			continue
		}
		// Beyond the open-pull-request paging bound: enough to name it.
		prs = append(prs, &forgejo.PullRequest{Index: issue.Index, Title: issue.Title, HTMLURL: issue.HTMLURL})
	}
	return prs, nil
}

// prChecks resolves head commits to their combined commit-status state,
// caching by SHA: the sections overlap (the current branch's pull request is
// usually one you opened too) and each lookup is a request of its own.
type prChecks struct {
	client *forgejo.Client
	repo   cmdutil.Repo
	cache  map[string]string
}

// state returns the combined state of the head commit's statuses, or the
// empty string when the commit has none — a commit nothing reported on must
// not be rendered as passing — or when the lookup failed.
func (c *prChecks) state(pr *forgejo.PullRequest) string {
	if pr.Head == nil || pr.Head.Sha == "" {
		return ""
	}
	if state, ok := c.cache[pr.Head.Sha]; ok {
		return state
	}

	state := ""
	status, _, err := c.client.GetCombinedStatus(c.repo.Owner, c.repo.Name, pr.Head.Sha)
	switch {
	case err != nil:
		debug.Logf(2, "combined status for %s (non-fatal): %v", pr.Head.Sha, err)
	case status != nil && status.TotalCount > 0:
		state = string(status.State)
	}
	c.cache[pr.Head.Sha] = state
	return state
}

func statusEntries(prs []*forgejo.PullRequest, checks *prChecks) []prStatusEntry {
	entries := make([]prStatusEntry, 0, len(prs))
	for _, pr := range prs {
		entry := prStatusEntry{
			Number:  pr.Index,
			Title:   pr.Title,
			HTMLURL: pr.HTMLURL,
			Checks:  checks.state(pr),
		}
		if pr.Head != nil {
			entry.Branch = pr.Head.Ref
		}
		entries = append(entries, entry)
	}
	return entries
}

// sectionLines renders a section's entries, each row optionally followed by
// its checks line, capped at statusLimit with a tail counting the remainder.
func sectionLines(entries []prStatusEntry) []string {
	shown := entries[:min(len(entries), statusLimit)]
	lines := make([]string, 0, 2*len(shown)+1)
	for _, e := range shown {
		line := fmt.Sprintf("%s  %s",
			output.Colorize(output.Green, "#"+strconv.FormatInt(e.Number, 10)),
			output.Truncate(e.Title, 60))
		if e.Branch != "" {
			line += " " + output.Colorize(output.Cyan, "["+e.Branch+"]")
		}
		lines = append(lines, line)
		if c := checksLine(e.Checks); c != "" {
			lines = append(lines, c)
		}
	}
	if extra := len(entries) - len(shown); extra > 0 {
		lines = append(lines, output.Colorize(output.Gray, fmt.Sprintf("And %d more", extra)))
	}
	return lines
}

func checksLine(state string) string {
	switch forgejo.StatusState(state) {
	case "":
		return ""
	case forgejo.StatusSuccess:
		return output.Colorize(output.Green, "✓ Checks passing")
	case forgejo.StatusPending:
		return output.Colorize(output.Yellow, "- Checks pending")
	default:
		return output.Colorize(output.Red, "× Checks failing")
	}
}
