//go:build integration

package integration

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// TestIssueListJSONFields checks that --json keeps only the named fields and
// prints them compactly on a pipe, as gh does.
func TestIssueListJSONFields(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "-L", "100", "--json", "number,title")

	if strings.Count(strings.TrimSpace(out), "\n") != 0 {
		t.Errorf("piped output should be one line:\n%s", out)
	}
	var issues []map[string]any
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	var found bool
	for _, iss := range issues {
		if len(iss) != 2 || iss["number"] == nil || iss["title"] == nil {
			t.Errorf("want exactly number and title, got %v", iss)
		}
		found = found || iss["title"] == "First issue"
	}
	if !found {
		t.Errorf("First issue missing from:\n%s", out)
	}
}

// TestIssueJSONGhShape reads an issue through create, view and edit and
// checks the names and shapes against gh's.
func TestIssueJSONGhShape(t *testing.T) {
	repo := adminUser + "/test-repo"
	out := mustRunFJ(t, "issue", "create", "-R", repo, "--title", "gh shape target",
		"--label", "bug", "--milestone", "v1.0", "--json", "number,isPinned,comments")
	var created struct {
		Number   int64 `json:"number"`
		IsPinned *bool `json:"isPinned"`
		Comments []any `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if created.Number == 0 || created.IsPinned == nil || *created.IsPinned || created.Comments == nil {
		t.Errorf("create: want a number, isPinned false and comments [], got %s", out)
	}
	num := strconv.FormatInt(created.Number, 10)

	if _, _, err := testClient.CreateIssueComment(adminUser, "test-repo", created.Number,
		forgejo.CreateIssueCommentOption{Body: "shape comment"}); err != nil {
		t.Fatalf("commenting: %v", err)
	}
	mustRunFJ(t, "api", "-X", "POST", "-R", repo, "repos/{owner}/{repo}/issues/"+num+"/pin")

	out = mustRunFJ(t, "issue", "view", num, "-R", repo, "--json",
		"assignees,author,closedAt,comments,isPinned,labels,milestone,state,url")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	keys := slices.Sorted(maps.Keys(got))
	if strings.Join(keys, ",") != "assignees,author,closedAt,comments,isPinned,labels,milestone,state,url" {
		t.Errorf("keys = %v", keys)
	}

	author, _ := got["author"].(map[string]any)
	if author["login"] != adminUser || author["is_bot"] != false || author["id"] == nil || author["name"] == nil {
		t.Errorf("author = %v, want gh's {id, is_bot, login, name}", author)
	}
	labels, _ := got["labels"].([]any)
	if len(labels) != 1 {
		t.Fatalf("labels = %v, want one", got["labels"])
	}
	if l := labels[0].(map[string]any); l["name"] != "bug" || l["color"] != "ee0701" || l["id"] == nil {
		t.Errorf("label = %v, want name bug and color ee0701", l)
	}
	if ms, _ := got["milestone"].(map[string]any); ms["title"] != "v1.0" || ms["number"] == nil {
		t.Errorf("milestone = %v", got["milestone"])
	}
	if got["state"] != "OPEN" || got["closedAt"] != nil || got["isPinned"] != true {
		t.Errorf("state %v, closedAt %v, isPinned %v; want OPEN, null, true", got["state"], got["closedAt"], got["isPinned"])
	}
	if url, _ := got["url"].(string); !strings.HasSuffix(url, "/"+repo+"/issues/"+num) {
		t.Errorf("url = %q", url)
	}
	if a, ok := got["assignees"].([]any); !ok || len(a) != 0 {
		t.Errorf("assignees = %v, want []", got["assignees"])
	}
	comments, _ := got["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("comments = %v, want one", got["comments"])
	}
	c := comments[0].(map[string]any)
	if url, _ := c["url"].(string); c["body"] != "shape comment" || url == "" || c["id"] == nil ||
		c["createdAt"] == nil || c["author"].(map[string]any)["login"] != adminUser {
		t.Errorf("comment = %v, want gh's {author, body, createdAt, id, url}", c)
	}

	out = mustRunFJ(t, "issue", "edit", num, "-R", repo, "--add-label", "documentation",
		"--json", "labels", "--jq", "[.labels[].name] | sort | join(\",\")")
	if strings.TrimSpace(out) != "bug,documentation" {
		t.Errorf("edit --jq = %q, want bug,documentation", out)
	}
}

// TestIssueJSONFlagErrors pins gh's messages and exit code 1 for misuse of
// the formatting flags.
func TestIssueJSONFlagErrors(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"--json"}, []string{"Specify one or more comma-separated fields for `--json`:", "\n  number\n"}},
		{[]string{"--json", "bogus"}, []string{`Unknown JSON field: "bogus"`, "Available fields:", "\n  title\n"}},
		{[]string{"--jq", ".[].title"}, []string{"cannot use `--jq` without specifying `--json`"}},
		{[]string{"--template", "{{.}}"}, []string{"cannot use `--template` without specifying `--json`"}},
	}
	for _, tt := range tests {
		args := append([]string{"issue", "list", "-R", adminUser + "/test-repo"}, tt.args...)
		stdout, stderr, err := runFJ(args...)
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Errorf("%v: err = %v, want exit 1", tt.args, err)
		}
		if stdout != "" {
			t.Errorf("%v: stdout = %q, want nothing", tt.args, stdout)
		}
		for _, w := range tt.want {
			if !strings.Contains(stderr, w) {
				t.Errorf("%v: stderr missing %q:\n%s", tt.args, w, stderr)
			}
		}
	}
}

func TestIssueListJQ(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "-L", "100", "--json", "title", "--jq", ".[].title")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if !slices.Contains(lines, "First issue") || !slices.Contains(lines, "Second issue") {
		t.Errorf("want one raw title per line, got:\n%s", out)
	}
}

func TestIssueListTemplate(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "-L", "100", "--json", "number,title",
		"--template", `{{range .}}{{if eq .title "First issue"}}{{tablerow (printf "#%v" .number) .title}}{{end}}{{end}}`)
	if !strings.Contains(out, "#1  First issue\n") {
		t.Errorf("template output missing the #1 row:\n%s", out)
	}
}

func TestIssueListJSONEmpty(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/another-repo", "--state", "all", "--json", "number")
	if out != "[]\n" {
		t.Errorf("empty list = %q, want []", out)
	}
}

// TestPRListJSONBare checks that a bare --json on pr lists the fields and
// exits 1, as gh does, instead of printing Forgejo's JSON.
func TestPRListJSONBare(t *testing.T) {
	stdout, stderr, err := runFJ("pr", "list", "-R", adminUser+"/test-repo", "--json")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Errorf("err = %v, want exit 1", err)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
	for _, w := range []string{"Specify one or more comma-separated fields for `--json`:", "\n  headRefName\n", "\n  reviewDecision\n"} {
		if !strings.Contains(stderr, w) {
			t.Errorf("stderr missing %q:\n%s", w, stderr)
		}
	}
}

func TestPRListJQ(t *testing.T) {
	out := mustRunFJ(t, "pr", "list", "-R", adminUser+"/test-repo", "--json", "number,title",
		"--jq", `.[] | select(.title == "Add feature-1") | .number`)
	if strings.TrimSpace(out) != "4" {
		t.Errorf("--jq = %q, want 4", out)
	}
}

func TestPRListTemplate(t *testing.T) {
	out := mustRunFJ(t, "pr", "list", "-R", adminUser+"/test-repo", "--json", "number,title",
		"--template", `{{range .}}{{if eq .title "Add feature-1"}}{{tablerow (printf "#%v" .number) .title}}{{end}}{{end}}`)
	if !strings.Contains(out, "#4  Add feature-1\n") {
		t.Errorf("template output missing the #4 row:\n%s", out)
	}
}

// TestPRCreateNoTemplate checks that pr create has no --template: -t is
// --title there, and gh's pr create --template means a PR template.
func TestPRCreateNoTemplate(t *testing.T) {
	_, stderr, err := runFJ("pr", "create", "-R", adminUser+"/test-repo", "--title", "x", "--template", "x")
	if err == nil || !strings.Contains(stderr, "unknown flag: --template") {
		t.Errorf("err = %v, stderr = %q; want an unknown flag error", err, stderr)
	}
}

// TestPRJSONGhShape creates a pull request with a real diff through fj,
// reviews it with an inline comment, and checks the names and shapes of the
// pull request, review and inline comment output against gh's.
func TestPRJSONGhShape(t *testing.T) {
	repo := adminUser + "/test-repo"
	readme, _, err := testClient.GetContents(adminUser, "test-repo", "main", "README.md")
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	// Rewriting README.md gives the diff both additions and deletions.
	if _, _, err := testClient.UpdateFile(adminUser, "test-repo", "README.md", forgejo.UpdateFileOptions{
		FileOptions: forgejo.FileOptions{Message: "Rewrite the readme\n\nWhy it is needed.", NewBranchName: "gh-shape"},
		SHA:         readme.SHA,
		Content:     base64.StdEncoding.EncodeToString([]byte("one\ntwo\n")),
	}); err != nil {
		t.Fatalf("committing on gh-shape: %v", err)
	}

	out := mustRunFJ(t, "pr", "create", "-R", repo, "--title", "gh shape PR", "--head", "gh-shape",
		"--base", "main", "--label", "bug", "--json", "number,isDraft")
	var created struct {
		Number  int64 `json:"number"`
		IsDraft *bool `json:"isDraft"`
	}
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if created.Number == 0 || created.IsDraft == nil || *created.IsDraft {
		t.Fatalf("create: want a number and isDraft false, got %s", out)
	}
	num := strconv.FormatInt(created.Number, 10)
	// Closed afterwards so later tests that pick an open PR do not get this one.
	t.Cleanup(func() {
		closed := forgejo.StateClosed
		_, _, _ = testClient.EditPullRequest(adminUser, "test-repo", created.Number, forgejo.EditPullRequestOption{State: &closed})
	})

	out = mustRunFJ(t, "pr", "view", num, "-R", repo, "--json",
		"additions,author,baseRefName,changedFiles,commits,deletions,files,headRefName,isDraft,labels,mergeable,state,url")
	var got struct {
		Additions    int    `json:"additions"`
		Deletions    int    `json:"deletions"`
		ChangedFiles int    `json:"changedFiles"`
		BaseRefName  string `json:"baseRefName"`
		HeadRefName  string `json:"headRefName"`
		IsDraft      bool   `json:"isDraft"`
		Mergeable    string `json:"mergeable"`
		State        string `json:"state"`
		URL          string `json:"url"`
		Author       struct {
			Login string `json:"login"`
			IsBot *bool  `json:"is_bot"`
		} `json:"author"`
		Labels []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"labels"`
		Commits []struct {
			OID             string `json:"oid"`
			MessageHeadline string `json:"messageHeadline"`
			MessageBody     string `json:"messageBody"`
			AuthoredDate    string `json:"authoredDate"`
			Authors         []struct {
				Login string `json:"login"`
				Email string `json:"email"`
			} `json:"authors"`
		} `json:"commits"`
		Files []struct {
			Path       string `json:"path"`
			Additions  int    `json:"additions"`
			Deletions  int    `json:"deletions"`
			ChangeType string `json:"changeType"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.State != "OPEN" || got.HeadRefName != "gh-shape" || got.BaseRefName != "main" || got.IsDraft {
		t.Errorf("state %q, head %q, base %q, isDraft %v; want OPEN, gh-shape, main, false",
			got.State, got.HeadRefName, got.BaseRefName, got.IsDraft)
	}
	if got.Author.Login != adminUser || got.Author.IsBot == nil {
		t.Errorf("author = %+v, want gh's author shape with login %s", got.Author, adminUser)
	}
	if !slices.Contains([]string{"MERGEABLE", "CONFLICTING", "UNKNOWN"}, got.Mergeable) {
		t.Errorf("mergeable = %q, want gh's enum", got.Mergeable)
	}
	if len(got.Labels) != 1 || got.Labels[0].Name != "bug" || got.Labels[0].Color != "ee0701" {
		t.Errorf("labels = %+v, want bug with color ee0701", got.Labels)
	}
	if !strings.HasSuffix(got.URL, "/"+repo+"/pulls/"+num) {
		t.Errorf("url = %q", got.URL)
	}
	if got.Additions == 0 || got.Deletions == 0 || got.ChangedFiles != 1 {
		t.Errorf("additions %d, deletions %d, changedFiles %d; want both > 0 and one file",
			got.Additions, got.Deletions, got.ChangedFiles)
	}
	if len(got.Commits) != 1 {
		t.Fatalf("commits = %+v, want one", got.Commits)
	}
	c := got.Commits[0]
	if len(c.OID) != 40 || c.MessageHeadline != "Rewrite the readme" || c.MessageBody != "Why it is needed." ||
		c.AuthoredDate == "" || len(c.Authors) != 1 || c.Authors[0].Login != adminUser || c.Authors[0].Email == "" {
		t.Errorf("commit = %+v, want gh's commit shape", c)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "README.md" || got.Files[0].ChangeType != "MODIFIED" ||
		got.Files[0].Additions != 2 || got.Files[0].Deletions == 0 {
		t.Errorf("files = %+v, want README.md MODIFIED with 2 additions", got.Files)
	}

	out = mustRunFJ(t, "pr", "review", "create", num, "-R", repo, "--comment", "--body", "shape review",
		"--comment-path", "README.md", "--comment-line", "1", "--comment-body", "shape inline", "--json", "id,state")
	var review struct {
		ID    int64  `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &review); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if review.ID == 0 || review.State != "COMMENTED" {
		t.Errorf("review create = %s, want an id and state COMMENTED", out)
	}

	out = mustRunFJ(t, "pr", "review", "list", num, "-R", repo, "--json", "author,body,commit,id,state,submittedAt")
	var reviews []struct {
		ID     int64  `json:"id"`
		Body   string `json:"body"`
		State  string `json:"state"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Commit struct {
			OID string `json:"oid"`
		} `json:"commit"`
		SubmittedAt string `json:"submittedAt"`
	}
	if err := json.Unmarshal([]byte(out), &reviews); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(reviews) != 1 || reviews[0].ID != review.ID || reviews[0].Body != "shape review" ||
		reviews[0].State != "COMMENTED" || reviews[0].Author.Login != adminUser ||
		len(reviews[0].Commit.OID) != 40 || reviews[0].SubmittedAt == "" {
		t.Errorf("reviews = %+v, want the one review in gh's shape", reviews)
	}

	out = mustRunFJ(t, "pr", "review", "comment", "list", num, "-R", repo, "--json", "body,line,path,reviewId")
	var comments []struct {
		ReviewID int64  `json:"reviewId"`
		Path     string `json:"path"`
		Line     int64  `json:"line"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal([]byte(out), &comments); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(comments) != 1 || comments[0].ReviewID != review.ID || comments[0].Path != "README.md" ||
		comments[0].Line != 1 || comments[0].Body != "shape inline" {
		t.Errorf("comments = %+v, want the inline comment with reviewId %d", comments, review.ID)
	}

	out = mustRunFJ(t, "pr", "view", num, "-R", repo, "--json", "latestReviews,reviewDecision,reviews",
		"--jq", `"\(.reviews | length) \(.latestReviews[0].state) [\(.reviewDecision)]"`)
	if strings.TrimSpace(out) != "1 COMMENTED []" {
		t.Errorf("reviews, latest state, decision = %q, want 1, COMMENTED and empty", out)
	}
}

// TestPRMergeJSON checks that a merged pull request reads as gh's MERGED.
func TestPRMergeJSON(t *testing.T) {
	if _, _, err := testClient.CreateFile(adminUser, "test-repo", "gh-merged.txt", forgejo.CreateFileOptions{
		FileOptions: forgejo.FileOptions{Message: "Add gh-merged", NewBranchName: "gh-merged"},
		Content:     base64.StdEncoding.EncodeToString([]byte("merged\n")),
	}); err != nil {
		t.Fatalf("committing on gh-merged: %v", err)
	}
	pr, _, err := testClient.CreatePullRequest(adminUser, "test-repo", forgejo.CreatePullRequestOption{
		Head: "gh-merged", Base: "main", Title: "gh merged PR",
	})
	if err != nil {
		t.Fatalf("creating PR: %v", err)
	}

	// Forgejo checks mergeability in the background and refuses a merge
	// until it is done.
	var out, stderr string
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(500 * time.Millisecond) {
		out, stderr, err = runFJ("pr", "merge", strconv.FormatInt(pr.Index, 10), "-R", adminUser+"/test-repo",
			"--json", "closed,mergeCommit,mergeable,mergedAt,mergedBy,state")
		if err == nil || time.Now().After(deadline) {
			break
		}
	}
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, stderr)
	}
	var got struct {
		State       string `json:"state"`
		Closed      bool   `json:"closed"`
		Mergeable   string `json:"mergeable"`
		MergedAt    string `json:"mergedAt"`
		MergeCommit *struct {
			OID string `json:"oid"`
		} `json:"mergeCommit"`
		MergedBy struct {
			Login string `json:"login"`
		} `json:"mergedBy"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.State != "MERGED" || !got.Closed || got.MergedAt == "" || got.MergedBy.Login != adminUser {
		t.Errorf("state %q, closed %v, mergedAt %q, mergedBy %q; want MERGED, true, a time and %s",
			got.State, got.Closed, got.MergedAt, got.MergedBy.Login, adminUser)
	}
	if got.Mergeable != "UNKNOWN" {
		t.Errorf("mergeable = %q, want UNKNOWN for a merged PR, as gh", got.Mergeable)
	}
	if got.MergeCommit == nil || len(got.MergeCommit.OID) != 40 {
		t.Errorf("mergeCommit = %+v, want an oid", got.MergeCommit)
	}
}
