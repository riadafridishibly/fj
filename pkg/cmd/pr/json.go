package pr

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/pr/review"
)

// prFields are gh's pull request JSON fields that Forgejo can fill. Left
// out: autoMergeRequest, closingIssuesReferences, mergeStateStatus,
// potentialMergeCommit, projectCards, projectItems, reactionGroups and
// statusCheckRollup.
var prFields = []string{
	"additions", "assignees", "author", "baseRefName", "baseRefOid", "body",
	"changedFiles", "closed", "closedAt", "comments", "commits", "createdAt",
	"deletions", "files", "fullDatabaseId", "headRefName", "headRefOid",
	"headRepository", "headRepositoryOwner", "id", "isCrossRepository", "isDraft",
	"labels", "latestReviews", "maintainerCanModify", "mergeCommit", "mergeable",
	"mergedAt", "mergedBy", "milestone", "number", "reviewDecision", "reviewRequests",
	"reviews", "state", "title", "updatedAt", "url",
}

// apiPullRequest is forgejo.PullRequest plus the fields the SDK does not
// decode.
type apiPullRequest struct {
	forgejo.PullRequest
	Draft                   bool            `json:"draft"`
	Additions               int             `json:"additions"`
	Deletions               int             `json:"deletions"`
	ChangedFiles            int             `json:"changed_files"`
	RequestedReviewers      []*forgejo.User `json:"requested_reviewers"`
	RequestedReviewersTeams []*forgejo.Team `json:"requested_reviewers_teams"`
}

// getPR fetches a pull request with the fields the SDK drops.
func getPR(f *cmdutil.Factory, repo cmdutil.Repo, index int64) (*apiPullRequest, error) {
	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	pr := new(apiPullRequest)
	if err := client.GetJSON(repo.APIPath("/pulls/%d", index), pr); err != nil {
		return nil, fmt.Errorf("getting pull request: %w", err)
	}
	return pr, nil
}

// prJSON returns pr keyed by gh field name. comments, commits, files and
// the review fields cost a request each, so they are fetched only when
// asked for.
func prJSON(f *cmdutil.Factory, repo cmdutil.Repo, pr *apiPullRequest, j *cmdutil.JSONFlags) (map[string]any, error) {
	head, base := pr.Head, pr.Base
	if head == nil {
		head = &forgejo.PRBranchInfo{}
	}
	if base == nil {
		base = &forgejo.PRBranchInfo{}
	}
	var headRepo, headOwner map[string]any
	if r := head.Repository; r != nil {
		headRepo = map[string]any{"id": r.ID, "name": r.Name, "nameWithOwner": r.FullName}
		if r.Owner != nil {
			headOwner = map[string]any{"id": r.Owner.ID, "login": r.Owner.UserName}
		}
	}
	var mergeCommit map[string]any
	if pr.MergedCommitID != nil && *pr.MergedCommitID != "" {
		mergeCommit = map[string]any{"oid": *pr.MergedCommitID}
	}
	requests := []map[string]any{}
	for _, u := range pr.RequestedReviewers {
		requests = append(requests, map[string]any{"__typename": "User", "login": u.UserName})
	}
	for _, t := range pr.RequestedReviewersTeams {
		requests = append(requests, map[string]any{"__typename": "Team", "name": t.Name})
	}

	m := map[string]any{
		"additions":           pr.Additions,
		"assignees":           cmdutil.JSONUsers(pr.Assignees),
		"author":              ghUser(pr.Poster),
		"baseRefName":         base.Ref,
		"baseRefOid":          base.Sha,
		"body":                pr.Body,
		"changedFiles":        pr.ChangedFiles,
		"closed":              pr.State == forgejo.StateClosed,
		"closedAt":            cmdutil.JSONTime(pr.Closed),
		"createdAt":           cmdutil.JSONTime(pr.Created),
		"deletions":           pr.Deletions,
		"fullDatabaseId":      strconv.FormatInt(pr.ID, 10),
		"headRefName":         head.Ref,
		"headRefOid":          head.Sha,
		"headRepository":      headRepo,
		"headRepositoryOwner": headOwner,
		"id":                  pr.ID,
		"isCrossRepository":   head.RepoID != base.RepoID,
		"isDraft":             pr.Draft,
		"labels":              cmdutil.JSONLabels(pr.Labels),
		"maintainerCanModify": pr.AllowMaintainerEdit,
		"mergeCommit":         mergeCommit,
		"mergeable":           mergeable(pr),
		"mergedAt":            cmdutil.JSONTime(pr.Merged),
		"mergedBy":            ghUser(pr.MergedBy),
		"milestone":           cmdutil.JSONMilestone(pr.Milestone),
		"number":              pr.Index,
		"reviewRequests":      requests,
		"state":               prState(pr),
		"title":               pr.Title,
		"updatedAt":           cmdutil.JSONTime(pr.Updated),
		"url":                 pr.HTMLURL,
	}

	wantReviews := j.Has("reviews") || j.Has("latestReviews") || j.Has("reviewDecision")
	if !j.Has("comments") && !j.Has("commits") && !j.Has("files") && !wantReviews {
		return m, nil
	}
	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	if j.Has("comments") {
		if m["comments"], err = cmdutil.JSONComments(client, repo, pr.Index); err != nil {
			return nil, err
		}
	}
	if j.Has("commits") {
		if m["commits"], err = prCommits(client, repo, pr.Index); err != nil {
			return nil, err
		}
	}
	if j.Has("files") {
		files, err := listAll[forgejo.ChangedFile](client, repo.APIPath("/pulls/%d/files", pr.Index))
		if err != nil {
			return nil, fmt.Errorf("listing files: %w", err)
		}
		out := make([]map[string]any, len(files))
		for i, file := range files {
			// Forgejo's "changed" is a content change, gh's MODIFIED; gh's
			// CHANGED is a file type change.
			change := strings.ToUpper(file.Status)
			if change == "CHANGED" {
				change = "MODIFIED"
			}
			out[i] = map[string]any{
				"path": file.Filename, "additions": file.Additions, "deletions": file.Deletions,
				"changeType": change,
			}
		}
		m["files"] = out
	}
	if wantReviews {
		reviews, err := listAll[*forgejo.PullReview](client, repo.APIPath("/pulls/%d/reviews", pr.Index))
		if err != nil {
			return nil, fmt.Errorf("listing reviews: %w", err)
		}
		// A REQUEST_REVIEW entry is a request, not a review.
		reviews = slices.DeleteFunc(reviews, func(r *forgejo.PullReview) bool {
			return r.State == forgejo.ReviewStateRequestReview
		})
		latest := latestReviews(reviews)
		m["reviews"], m["latestReviews"] = reviewsJSON(reviews), reviewsJSON(latest)
		m["reviewDecision"] = reviewDecision(reviews)
	}
	return m, nil
}

func reviewsJSON(reviews []*forgejo.PullReview) []map[string]any {
	out := make([]map[string]any, len(reviews))
	for i, r := range reviews {
		out[i] = review.JSON(r)
	}
	return out
}

// ghUser is gh's author shape, which adds is_bot to JSONUser.
func ghUser(u *forgejo.User) map[string]any {
	m := cmdutil.JSONUser(u)
	if m != nil {
		m["is_bot"] = false
	}
	return m
}

// prState is gh's state: MERGED, OPEN or CLOSED.
func prState(pr *apiPullRequest) string {
	if pr.HasMerged {
		return "MERGED"
	}
	return strings.ToUpper(string(pr.State))
}

// mergeable maps Forgejo's boolean to gh's enum. Merged and closed PRs read
// as UNKNOWN, as in gh. Forgejo's false covers a conflict, a check still
// running and a draft, so a draft reads as UNKNOWN too.
func mergeable(pr *apiPullRequest) string {
	switch {
	case pr.State != forgejo.StateOpen:
		return "UNKNOWN"
	case pr.Mergeable:
		return "MERGEABLE"
	case !pr.Draft:
		return "CONFLICTING"
	}
	return "UNKNOWN"
}

// latestReviews keeps each author's last review, in the order of those
// last reviews.
func latestReviews(reviews []*forgejo.PullReview) []*forgejo.PullReview {
	author := func(r *forgejo.PullReview) int64 {
		if r.Reviewer == nil {
			return 0
		}
		return r.Reviewer.ID
	}
	last := map[int64]int{}
	for i, r := range reviews {
		last[author(r)] = i
	}
	var out []*forgejo.PullReview
	for i, r := range reviews {
		if last[author(r)] == i {
			out = append(out, r)
		}
	}
	return out
}

// reviewDecision is gh's reviewDecision, taken from each author's latest
// approval or change request. Dismissed reviews and comments do not count,
// so a comment after a change request leaves the request standing. It is
// never REVIEW_REQUIRED, since Forgejo does not expose whether branch
// protection requires a review.
func reviewDecision(reviews []*forgejo.PullReview) string {
	opinions := slices.DeleteFunc(slices.Clone(reviews), func(r *forgejo.PullReview) bool {
		return r.Dismissed || r.State != forgejo.ReviewStateApproved && r.State != forgejo.ReviewStateRequestChanges
	})
	decision := ""
	for _, r := range latestReviews(opinions) {
		if r.State == forgejo.ReviewStateRequestChanges {
			return "CHANGES_REQUESTED"
		}
		decision = "APPROVED"
	}
	return decision
}

// apiCommit is the part of Forgejo's commit that gh's commit shape needs.
type apiCommit struct {
	SHA    string        `json:"sha"`
	Author *forgejo.User `json:"author"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

// prCommits lists the pull request's commits in gh's shape. files and
// verification are turned off; gh's shape needs neither.
func prCommits(client *api.Client, repo cmdutil.Repo, index int64) ([]map[string]any, error) {
	commits, err := listAll[apiCommit](client, repo.APIPath("/pulls/%d/commits?files=false&verification=false", index))
	if err != nil {
		return nil, fmt.Errorf("listing commits: %w", err)
	}
	out := make([]map[string]any, len(commits))
	for i, c := range commits {
		a := c.Commit.Author
		author := map[string]any{"email": a.Email, "name": a.Name, "id": nil, "login": ""}
		if c.Author != nil {
			author["id"], author["login"] = c.Author.ID, c.Author.UserName
		}
		headline, body := splitMessage(c.Commit.Message)
		out[i] = map[string]any{
			"oid": c.SHA, "messageHeadline": headline, "messageBody": body,
			"authoredDate": cmdutil.JSONTime(&a.Date), "committedDate": cmdutil.JSONTime(&c.Commit.Committer.Date),
			"authors": []map[string]any{author},
		}
	}
	return out, nil
}

// splitMessage splits a commit message into gh's headline and body.
func splitMessage(msg string) (headline, body string) {
	headline, body, _ = strings.Cut(msg, "\n")
	return strings.TrimSpace(headline), strings.TrimSpace(body)
}

// listAll pages through a list endpoint until a short page.
func listAll[T any](client *api.Client, path string) ([]T, error) {
	const limit = 50
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	var all []T
	for page := 1; ; page++ {
		var items []T
		if err := client.GetJSON(fmt.Sprintf("%s%spage=%d&limit=%d", path, sep, page, limit), &items); err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < limit {
			return all, nil
		}
	}
}

// writePR prints the pull request after a write. The write's response is
// the SDK struct, which lacks draft and the diff stats, so the PR is fetched
// again.
func writePR(f *cmdutil.Factory, repo cmdutil.Repo, index int64, j *cmdutil.JSONFlags) error {
	pr, err := getPR(f, repo, index)
	if err != nil {
		return err
	}
	data, err := prJSON(f, repo, pr, j)
	if err != nil {
		return err
	}
	return j.Write(os.Stdout, data)
}
