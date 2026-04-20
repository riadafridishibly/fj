package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/git"
	"github.com/riadafridishibly/fj/internal/output"
)

type statusOptions struct {
	Factory    *cmdutil.Factory
	JSONOutput bool
}

func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	opts := &statusOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show repository status overview",
		Long:  "Display a summary of the repository including issue/PR counts and current branch status.",
		Example: `  $ fj status
  $ fj status --json`,
		Aliases: []string{"st"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(opts)
		},
	}

	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

type repoStatus struct {
	Repository    string `json:"repository"`
	DefaultBranch string `json:"default_branch"`
	Stars         int    `json:"stars"`
	Forks         int    `json:"forks"`
	Watchers      int    `json:"watchers"`

	OpenIssues   int `json:"open_issues"`
	ClosedIssues int `json:"closed_issues"`

	OpenPRs   int `json:"open_prs"`
	ClosedPRs int `json:"closed_prs"`
	MergedPRs int `json:"merged_prs"`

	LatestRelease *releaseStatus `json:"latest_release,omitempty"`

	Branch *branchStatus `json:"branch,omitempty"`
}

type releaseStatus struct {
	Tag         string    `json:"tag"`
	Title       string    `json:"title"`
	IsDraft     bool      `json:"draft"`
	IsPrerelease bool     `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

type branchStatus struct {
	Name         string    `json:"name"`
	ExistsRemote bool      `json:"exists_on_remote"`
	PR           *prStatus `json:"pull_request,omitempty"`
}

type prStatus struct {
	Number       int64  `json:"number"`
	Title        string `json:"title"`
	State        string `json:"state"`
	Mergeable    bool   `json:"mergeable"`
	Draft        bool   `json:"draft"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	ChangedFiles int    `json:"changed_files"`
	HTMLURL      string `json:"html_url"`
	HeadSHA      string `json:"head_sha"`
	LocalSHA     string `json:"local_sha"`
	Synced       bool   `json:"synced"`
}

func statusRun(opts *statusOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Get repo info
	r, _, err := client.GetRepo(repo.Owner, repo.Name)
	if err != nil {
		return fmt.Errorf("getting repository: %w", err)
	}

	status := &repoStatus{
		Repository:    r.FullName,
		DefaultBranch: r.DefaultBranch,
		Stars:         r.Stars,
		Forks:         r.Forks,
		Watchers:      r.Watchers,
	}

	// Get issue counts (open + closed) using minimal page size, reading X-Total-Count
	_, respOpenIssues, err := client.ListRepoIssues(repo.Owner, repo.Name, forgejo.ListIssueOption{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: 1},
		State:       forgejo.StateOpen,
		Type:        forgejo.IssueTypeIssue,
	})
	if err != nil {
		return fmt.Errorf("listing open issues: %w", err)
	}
	status.OpenIssues = cmdutil.TotalCount(respOpenIssues)

	_, respClosedIssues, err := client.ListRepoIssues(repo.Owner, repo.Name, forgejo.ListIssueOption{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: 1},
		State:       forgejo.StateClosed,
		Type:        forgejo.IssueTypeIssue,
	})
	if err != nil {
		return fmt.Errorf("listing closed issues: %w", err)
	}
	status.ClosedIssues = cmdutil.TotalCount(respClosedIssues)

	// Get PR counts - fetch open PRs (also scan for current branch PR)
	branch, _ := git.CurrentBranch()

	openPRs, respOpenPRs, err := client.ListRepoPullRequests(repo.Owner, repo.Name, forgejo.ListPullRequestsOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: 50},
		State:       forgejo.StateOpen,
	})
	if err != nil {
		return fmt.Errorf("listing open PRs: %w", err)
	}
	status.OpenPRs = cmdutil.TotalCount(respOpenPRs)

	// Find PR for current branch among open PRs
	var branchPR *forgejo.PullRequest
	for _, pr := range openPRs {
		if pr.Head != nil && pr.Head.Ref == branch {
			branchPR = pr
			break
		}
	}

	// Get closed PRs to count merged vs closed
	closedTotal, merged, err := countClosedAndMergedPRs(client, repo)
	if err != nil {
		return fmt.Errorf("counting closed PRs: %w", err)
	}
	status.MergedPRs = merged
	status.ClosedPRs = closedTotal - merged

	// Latest release (non-fatal if repo has none)
	if latest, _, err := client.GetLatestRelease(repo.Owner, repo.Name); err == nil && latest != nil {
		title := latest.Title
		if title == "" {
			title = latest.TagName
		}
		status.LatestRelease = &releaseStatus{
			Tag:          latest.TagName,
			Title:        title,
			IsDraft:      latest.IsDraft,
			IsPrerelease: latest.IsPrerelease,
			PublishedAt:  latest.PublishedAt,
			HTMLURL:      latest.HTMLURL,
		}
	}

	// Current branch info
	if branch != "" {
		bs := &branchStatus{Name: branch}

		// Check if branch exists on remote
		_, _, err := client.GetRepoBranch(repo.Owner, repo.Name, branch)
		bs.ExistsRemote = err == nil

		// If we didn't find a PR in open PRs, also check closed/merged PRs for this branch
		if branchPR == nil {
			branchPR, err = findBranchPR(client, repo, branch)
			if err != nil {
				// Non-fatal, just skip PR info
				branchPR = nil
			}
		}

		if branchPR != nil {
			ps := &prStatus{
				Number:    branchPR.Index,
				Title:     branchPR.Title,
				Mergeable: branchPR.Mergeable,
				HTMLURL:   branchPR.HTMLURL,
			}

			if branchPR.HasMerged {
				ps.State = "merged"
			} else {
				ps.State = string(branchPR.State)
			}

			if branchPR.Head != nil {
				ps.HeadSHA = branchPR.Head.Sha
			}

			// Get local HEAD sha
			localSHA, _ := git.Run("rev-parse", "HEAD")
			ps.LocalSHA = localSHA
			ps.Synced = localSHA != "" && ps.HeadSHA != "" && localSHA == ps.HeadSHA

			// Get additions/deletions/draft via raw API (SDK doesn't expose these fields)
			additions, deletions, changedFiles, draft, err := fetchPRDetails(opts.Factory, repo, branchPR.Index)
			if err == nil {
				ps.Additions = additions
				ps.Deletions = deletions
				ps.ChangedFiles = changedFiles
				ps.Draft = draft
			}

			bs.PR = ps
		}

		status.Branch = bs
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	printStatus(status)
	return nil
}

func countClosedAndMergedPRs(client *forgejo.Client, repo cmdutil.Repo) (total, merged int, err error) {
	page := 1
	for {
		prs, resp, err := client.ListRepoPullRequests(repo.Owner, repo.Name, forgejo.ListPullRequestsOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: 50},
			State:       forgejo.StateClosed,
		})
		if err != nil {
			return 0, 0, err
		}
		if page == 1 {
			total = cmdutil.TotalCount(resp)
		}
		for _, pr := range prs {
			if pr.HasMerged {
				merged++
			}
		}
		if len(prs) < 50 {
			break
		}
		page++
	}
	return total, merged, nil
}

func findBranchPR(client *forgejo.Client, repo cmdutil.Repo, branch string) (*forgejo.PullRequest, error) {
	page := 1
	for {
		prs, _, err := client.ListRepoPullRequests(repo.Owner, repo.Name, forgejo.ListPullRequestsOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: 50},
			State:       forgejo.StateClosed,
		})
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			if pr.Head != nil && pr.Head.Ref == branch {
				return pr, nil
			}
		}
		if len(prs) < 50 {
			break
		}
		page++
	}
	return nil, nil
}

// fetchPRDetails gets additions/deletions/changed_files/draft via raw API call
// since the Forgejo SDK's PullRequest struct doesn't expose these fields.
func fetchPRDetails(f *cmdutil.Factory, repo cmdutil.Repo, prIndex int64) (additions, deletions, changedFiles int, draft bool, err error) {
	cfg, err := f.Config()
	if err != nil {
		return 0, 0, 0, false, err
	}

	token, err := cfg.TokenForHost(repo.Host)
	if err != nil {
		return 0, 0, 0, false, err
	}

	scheme := "https"
	if os.Getenv("FJ_INSECURE") != "" {
		scheme = "http"
	}

	url := fmt.Sprintf("%s://%s/api/v1/repos/%s/%s/pulls/%d",
		scheme, repo.Host, repo.Owner, repo.Name, prIndex)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, 0, false, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, 0, false, err
	}
	defer resp.Body.Close()

	var result struct {
		Additions    int  `json:"additions"`
		Deletions    int  `json:"deletions"`
		ChangedFiles int  `json:"changed_files"`
		Draft        bool `json:"draft"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, 0, false, err
	}
	return result.Additions, result.Deletions, result.ChangedFiles, result.Draft, nil
}

func printStatus(s *repoStatus) {
	w := os.Stdout

	// Repository header
	fmt.Fprintf(w, "%s\n", output.Colorize(output.Bold, s.Repository))
	fmt.Fprintf(w, "  Default branch: %s\n", s.DefaultBranch)
	fmt.Fprintf(w, "  Stars: %d  Forks: %d  Watchers: %d\n", s.Stars, s.Forks, s.Watchers)
	fmt.Fprintln(w)

	// Issues
	fmt.Fprintf(w, "%s\n", output.Colorize(output.Bold, "Issues"))
	fmt.Fprintf(w, "  %s open  %s closed\n",
		output.Colorize(output.Green, fmt.Sprintf("%d", s.OpenIssues)),
		output.Colorize(output.Red, fmt.Sprintf("%d", s.ClosedIssues)),
	)
	fmt.Fprintln(w)

	// Pull Requests
	fmt.Fprintf(w, "%s\n", output.Colorize(output.Bold, "Pull Requests"))
	fmt.Fprintf(w, "  %s open  %s closed  %s merged\n",
		output.Colorize(output.Green, fmt.Sprintf("%d", s.OpenPRs)),
		output.Colorize(output.Red, fmt.Sprintf("%d", s.ClosedPRs)),
		output.Colorize(output.Magenta, fmt.Sprintf("%d", s.MergedPRs)),
	)
	fmt.Fprintln(w)

	// Latest release
	if s.LatestRelease != nil {
		r := s.LatestRelease
		fmt.Fprintf(w, "%s\n", output.Colorize(output.Bold, "Latest Release"))
		typ := "release"
		typColor := output.Green
		switch {
		case r.IsDraft:
			typ = "draft"
			typColor = output.Yellow
		case r.IsPrerelease:
			typ = "pre-release"
			typColor = output.Magenta
		}
		fmt.Fprintf(w, "  %s %s %s\n",
			output.Colorize(output.Cyan, r.Tag),
			output.Truncate(r.Title, 50),
			output.Colorize(typColor, "("+typ+")"),
		)
		fmt.Fprintf(w, "  Published %s\n", output.RelativeTimeStr(r.PublishedAt))
		fmt.Fprintln(w)
	}

	// Current branch
	if s.Branch != nil {
		fmt.Fprintf(w, "%s\n", output.Colorize(output.Bold, "Current Branch"))
		fmt.Fprintf(w, "  %s", output.Colorize(output.Cyan, s.Branch.Name))
		if s.Branch.ExistsRemote {
			fmt.Fprintf(w, " %s", output.Colorize(output.Green, "(on remote)"))
		} else {
			fmt.Fprintf(w, " %s", output.Colorize(output.Yellow, "(local only)"))
		}
		fmt.Fprintln(w)

		if s.Branch.PR != nil {
			pr := s.Branch.PR
			stateColor := output.Green
			switch pr.State {
			case "merged":
				stateColor = output.Magenta
			case "closed":
				stateColor = output.Red
			}
			fmt.Fprintf(w, "  PR #%d: %s %s\n",
				pr.Number,
				output.Truncate(pr.Title, 50),
				output.Colorize(stateColor, "("+pr.State+")"),
			)

			// Diff stats
			var parts []string
			if pr.Additions > 0 || pr.Deletions > 0 {
				parts = append(parts,
					output.Colorize(output.Green, fmt.Sprintf("+%d", pr.Additions)),
					output.Colorize(output.Red, fmt.Sprintf("-%d", pr.Deletions)),
				)
			}
			if pr.ChangedFiles > 0 {
				files := "files"
				if pr.ChangedFiles == 1 {
					files = "file"
				}
				parts = append(parts, fmt.Sprintf("%d %s changed", pr.ChangedFiles, files))
			}
			if len(parts) > 0 {
				fmt.Fprintf(w, "  %s\n", strings.Join(parts, ", "))
			}

			// Sync status
			if pr.State == "open" {
				if pr.Synced {
					fmt.Fprintf(w, "  %s\n", output.Colorize(output.Green, "Local is up to date with PR"))
				} else {
					fmt.Fprintf(w, "  %s\n", output.Colorize(output.Yellow, "Local is out of sync with PR"))
				}
				if pr.Mergeable {
					fmt.Fprintf(w, "  %s\n", output.Colorize(output.Green, "Mergeable"))
				} else if pr.Draft {
					fmt.Fprintf(w, "  %s\n", output.Colorize(output.Yellow, "WIP / Draft"))
				} else {
					fmt.Fprintf(w, "  %s\n", output.Colorize(output.Red, "Has conflicts"))
				}
			}

			fmt.Fprintf(w, "  %s\n", output.Colorize(output.Gray, pr.HTMLURL))
		}
	}
}
