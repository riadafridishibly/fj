package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/debug"
	"github.com/riadafridishibly/fj/internal/git"
	"github.com/riadafridishibly/fj/internal/output"
)

type statusOptions struct {
	Factory    *cmdutil.Factory
	JSONOutput bool
	Full       bool
}

func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	opts := &statusOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show repository status overview",
		Long: `Display a summary of the repository including issue/PR counts and current branch status.

By default the "closed" PR count is the total from the server and includes merged PRs.
Pass --full to paginate every closed PR and produce a merged-vs-closed breakdown; this
can take tens of seconds on repositories with many closed PRs.`,
		Example: `  $ fj status
  $ fj status --json
  $ fj status --full`,
		Aliases: []string{"st"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(opts)
		},
	}

	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)
	cmd.Flags().BoolVar(&opts.Full, "full", false, "Compute the merged-vs-closed PR breakdown (slow on large repos)")

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
	// ClosedPRs is the total count of PRs in the "closed" state as reported by
	// the server (includes merged PRs). When --full is used, ClosedPRs is
	// re-computed as closed-only (merged excluded).
	ClosedPRs int `json:"closed_prs"`
	// MergedPRs is only populated when --full is used. nil otherwise.
	MergedPRs *int `json:"merged_prs,omitempty"`

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
	defer debug.Track(1, "fj status (total)")()

	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}
	debug.Logf(1, "resolved repo: %s/%s on %s", repo.Owner, repo.Name, repo.Host)

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Get repo info
	done := debug.Track(2, "GetRepo")
	r, _, err := client.GetRepo(repo.Owner, repo.Name)
	done()
	if err != nil {
		return fmt.Errorf("getting repository: %w", err)
	}
	debug.Logf(3, "repo default_branch=%s stars=%d forks=%d", r.DefaultBranch, r.Stars, r.Forks)

	status := &repoStatus{
		Repository:    r.FullName,
		DefaultBranch: r.DefaultBranch,
		Stars:         r.Stars,
		Forks:         r.Forks,
		Watchers:      r.Watchers,
	}

	// Get issue counts (open + closed) using minimal page size, reading X-Total-Count
	debug.Logf(1, "phase: issue counts")
	done = debug.Track(2, "ListRepoIssues open (count)")
	_, respOpenIssues, err := client.ListRepoIssues(repo.Owner, repo.Name, forgejo.ListIssueOption{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: 1},
		State:       forgejo.StateOpen,
		Type:        forgejo.IssueTypeIssue,
	})
	done()
	if err != nil {
		return fmt.Errorf("listing open issues: %w", err)
	}
	status.OpenIssues = cmdutil.TotalCount(respOpenIssues)
	debug.Logf(3, "open issues X-Total-Count=%d", status.OpenIssues)

	done = debug.Track(2, "ListRepoIssues closed (count)")
	_, respClosedIssues, err := client.ListRepoIssues(repo.Owner, repo.Name, forgejo.ListIssueOption{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: 1},
		State:       forgejo.StateClosed,
		Type:        forgejo.IssueTypeIssue,
	})
	done()
	if err != nil {
		return fmt.Errorf("listing closed issues: %w", err)
	}
	status.ClosedIssues = cmdutil.TotalCount(respClosedIssues)
	debug.Logf(3, "closed issues X-Total-Count=%d", status.ClosedIssues)

	// Get PR counts - fetch open PRs (also scan for current branch PR)
	branch, _ := git.CurrentBranch()
	debug.Logf(1, "phase: open PRs (current branch=%q)", branch)

	done = debug.Track(2, "ListRepoPullRequests open page=1 size=50")
	openPRs, respOpenPRs, err := client.ListRepoPullRequests(repo.Owner, repo.Name, forgejo.ListPullRequestsOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: 50},
		State:       forgejo.StateOpen,
	})
	done()
	if err != nil {
		return fmt.Errorf("listing open PRs: %w", err)
	}
	status.OpenPRs = cmdutil.TotalCount(respOpenPRs)
	debug.Logf(3, "open PRs X-Total-Count=%d, returned=%d", status.OpenPRs, len(openPRs))

	// Find PR for current branch among open PRs
	var branchPR *forgejo.PullRequest
	for _, pr := range openPRs {
		if pr.Head != nil && pr.Head.Ref == branch {
			branchPR = pr
			break
		}
	}
	if branchPR != nil {
		debug.Logf(3, "found open PR for branch %q: #%d", branch, branchPR.Index)
	}

	// Closed-PR count. Splitting merged vs. closed requires paginating every
	// closed PR (Forgejo has no filter or stats endpoint for it) which is slow
	// on busy repos, so gate it behind --full. Default: just the total.
	if opts.Full {
		debug.Logf(1, "phase: scan closed PRs (count merged — --full)")
		done = debug.Track(2, "scanClosedPRs")
		closedTotal, merged, err := scanClosedPRs(client, repo)
		done()
		if err != nil {
			return fmt.Errorf("scanning closed PRs: %w", err)
		}
		status.ClosedPRs = closedTotal - merged
		status.MergedPRs = &merged
		debug.Logf(3, "closed PRs total=%d merged=%d", closedTotal, merged)
	} else {
		debug.Logf(1, "phase: closed PRs count (X-Total-Count only)")
		done = debug.Track(2, "ListRepoPullRequests closed (count)")
		_, respClosedPRs, err := client.ListRepoPullRequests(repo.Owner, repo.Name, forgejo.ListPullRequestsOptions{
			ListOptions: forgejo.ListOptions{Page: 1, PageSize: 1},
			State:       forgejo.StateClosed,
		})
		done()
		if err != nil {
			return fmt.Errorf("counting closed PRs: %w", err)
		}
		status.ClosedPRs = cmdutil.TotalCount(respClosedPRs)
		debug.Logf(3, "closed PRs X-Total-Count=%d (merged breakdown skipped)", status.ClosedPRs)
	}

	// Latest release (non-fatal if repo has none)
	debug.Logf(1, "phase: latest release")
	done = debug.Track(2, "GetLatestRelease")
	latest, _, relErr := client.GetLatestRelease(repo.Owner, repo.Name)
	done()
	if relErr == nil && latest != nil {
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
		debug.Logf(1, "phase: current branch (%s)", branch)
		bs := &branchStatus{Name: branch}

		// Check if branch exists on remote
		done = debug.Track(2, "GetRepoBranch")
		_, _, err := client.GetRepoBranch(repo.Owner, repo.Name, branch)
		done()
		bs.ExistsRemote = err == nil
		debug.Logf(3, "branch exists on remote: %v", bs.ExistsRemote)

		// If we didn't find a PR among open PRs, try the base/head endpoint —
		// a single O(1) lookup instead of paginating closed PRs. We assume the
		// base is the repo's default branch, which covers the common case.
		// Also returns the full PR object, so no separate fetchPRDetails call.
		var prDetails *fullPR
		if branchPR == nil && branch != r.DefaultBranch {
			debug.Logf(1, "phase: lookup PR by base/head (%s <- %s)", r.DefaultBranch, branch)
			done = debug.Track(2, "getPullByBaseHead")
			fp, lookupErr := getPullByBaseHead(opts.Factory, repo, r.DefaultBranch, branch)
			done()
			switch {
			case lookupErr != nil:
				debug.Logf(2, "getPullByBaseHead error (non-fatal): %v", lookupErr)
			case fp != nil:
				branchPR = &fp.PullRequest
				prDetails = fp
				debug.Logf(3, "found PR #%d via base/head (state=%s merged=%v)", fp.Index, fp.State, fp.HasMerged)
			default:
				debug.Logf(3, "no PR found for base=%s head=%s", r.DefaultBranch, branch)
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

			// Populate extra fields (SDK PullRequest doesn't expose them). If we
			// got the PR from base/head, the response already has everything.
			if prDetails != nil {
				ps.Additions = prDetails.Additions
				ps.Deletions = prDetails.Deletions
				ps.ChangedFiles = prDetails.ChangedFiles
				ps.Draft = prDetails.Draft
			} else {
				done = debug.Track(2, fmt.Sprintf("fetchPRDetails #%d", branchPR.Index))
				additions, deletions, changedFiles, draft, err := fetchPRDetails(opts.Factory, repo, branchPR.Index)
				done()
				if err == nil {
					ps.Additions = additions
					ps.Deletions = deletions
					ps.ChangedFiles = changedFiles
					ps.Draft = draft
				} else {
					debug.Logf(2, "fetchPRDetails error (non-fatal): %v", err)
				}
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

// scanClosedPRs paginates through every closed PR to tally merged vs. closed.
// Forgejo has no cheaper endpoint for this breakdown.
func scanClosedPRs(client *forgejo.Client, repo cmdutil.Repo) (total, merged int, err error) {
	page := 1
	for {
		done := debug.Track(3, fmt.Sprintf("ListRepoPullRequests closed page=%d size=50", page))
		prs, resp, e := client.ListRepoPullRequests(repo.Owner, repo.Name, forgejo.ListPullRequestsOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: 50},
			State:       forgejo.StateClosed,
		})
		done()
		if e != nil {
			return 0, 0, e
		}
		if page == 1 {
			total = cmdutil.TotalCount(resp)
			debug.Logf(3, "closed PRs X-Total-Count=%d (will paginate)", total)
		}
		pageMerged := 0
		for _, pr := range prs {
			if pr.HasMerged {
				merged++
				pageMerged++
			}
		}
		debug.Logf(3, "page %d: returned=%d merged_on_page=%d running_merged=%d", page, len(prs), pageMerged, merged)
		if len(prs) < 50 {
			break
		}
		page++
	}
	return total, merged, nil
}

// fullPR mirrors the Forgejo PR API response, embedding the SDK type and
// adding the fields the SDK struct doesn't expose.
type fullPR struct {
	forgejo.PullRequest
	Additions    int  `json:"additions"`
	Deletions    int  `json:"deletions"`
	ChangedFiles int  `json:"changed_files"`
	Draft        bool `json:"draft"`
}

// getPullByBaseHead fetches a PR by its base/head branches via
// GET /repos/{owner}/{repo}/pulls/{base}/{head}. Returns (nil, nil) on 404.
func getPullByBaseHead(f *cmdutil.Factory, repo cmdutil.Repo, base, head string) (*fullPR, error) {
	cfg, err := f.Config()
	if err != nil {
		return nil, err
	}

	token, err := cfg.TokenForHost(repo.Host)
	if err != nil {
		return nil, err
	}

	scheme := "https"
	if os.Getenv("FJ_INSECURE") != "" {
		scheme = "http"
	}

	u := fmt.Sprintf("%s://%s/api/v1/repos/%s/%s/pulls/%s/%s",
		scheme, repo.Host, repo.Owner, repo.Name,
		url.PathEscape(base), url.PathEscape(head))

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}

	var fp fullPR
	if err := json.NewDecoder(resp.Body).Decode(&fp); err != nil {
		return nil, err
	}
	return &fp, nil
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
	if s.MergedPRs != nil {
		fmt.Fprintf(w, "  %s open  %s closed  %s merged\n",
			output.Colorize(output.Green, fmt.Sprintf("%d", s.OpenPRs)),
			output.Colorize(output.Red, fmt.Sprintf("%d", s.ClosedPRs)),
			output.Colorize(output.Magenta, fmt.Sprintf("%d", *s.MergedPRs)),
		)
	} else {
		fmt.Fprintf(w, "  %s open  %s closed %s\n",
			output.Colorize(output.Green, fmt.Sprintf("%d", s.OpenPRs)),
			output.Colorize(output.Red, fmt.Sprintf("%d", s.ClosedPRs)),
			output.Colorize(output.Gray, "(incl. merged — use --full for breakdown)"),
		)
	}
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
