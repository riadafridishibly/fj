//go:build integration

package integration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// commitReferencing creates an issue and a commit whose message mentions it,
// which is what makes Forgejo record a "referenced ... from a commit" event.
// The message deliberately avoids a closing keyword, so the reference stays a
// plain commit_ref rather than also closing the issue. Returns the issue index.
func commitReferencing(t *testing.T, title string) int64 {
	t.Helper()

	issue, _, err := testClient.CreateIssue(adminUser, "test-repo", forgejo.CreateIssueOption{
		Title: title,
		Body:  "created by the timeline integration test",
	})
	if err != nil {
		t.Fatalf("creating issue: %v", err)
	}

	_, _, err = testClient.CreateFile(adminUser, "test-repo",
		fmt.Sprintf("timeline-%d.txt", issue.Index),
		forgejo.CreateFileOptions{
			FileOptions: forgejo.FileOptions{
				Message: fmt.Sprintf("Touch a file, mentioning #%d", issue.Index),
			},
			Content: base64.StdEncoding.EncodeToString([]byte("hello\n")),
		})
	if err != nil {
		t.Fatalf("creating commit: %v", err)
	}
	return issue.Index
}

// viewUntil polls `fj <args>` until its output contains want. Forgejo resolves
// cross-references on a queue after the push returns, so the event is not
// guaranteed to exist by the time the commit call does.
func viewUntil(t *testing.T, want string, args ...string) string {
	t.Helper()

	var stdout string
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		stdout = mustRunFJ(t, args...)
		if strings.Contains(stdout, want) {
			return stdout
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in `fj %s`:\n%s", want, strings.Join(args, " "), stdout)
	return ""
}

// TestIssueViewTimelineCommitRef is the motivating case: Forgejo records
// "referenced this issue from a commit" as a timeline event, which the SDK
// cannot reach at all. It must show without asking for it.
func TestIssueViewTimelineCommitRef(t *testing.T) {
	index := commitReferencing(t, "Timeline commit ref target")
	num := strconv.FormatInt(index, 10)

	stdout := viewUntil(t, "referenced this issue from a commit",
		"issue", "view", num, "-R", adminUser+"/test-repo")

	if !strings.Contains(stdout, "--- Timeline") {
		t.Errorf("expected a Timeline section:\n%s", stdout)
	}
	if !strings.Contains(stdout, adminUser+" referenced this issue from a commit") {
		t.Errorf("expected the actor to precede the event:\n%s", stdout)
	}
}

// TestIssueViewTimelineDisabled checks the escape hatch: --show-timeline=false
// suppresses the section that is otherwise on by default.
func TestIssueViewTimelineDisabled(t *testing.T) {
	index := commitReferencing(t, "Timeline opt-out target")
	num := strconv.FormatInt(index, 10)

	// Only meaningful once the event exists, so wait for it to show first.
	viewUntil(t, "referenced this issue from a commit",
		"issue", "view", num, "-R", adminUser+"/test-repo")

	stdout := mustRunFJ(t, "issue", "view", num, "-R", adminUser+"/test-repo", "--show-timeline=false")

	if strings.Contains(stdout, "--- Timeline") || strings.Contains(stdout, "referenced this issue") {
		t.Errorf("--show-timeline=false should suppress the timeline:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Timeline opt-out target") {
		t.Errorf("the issue itself should still render:\n%s", stdout)
	}
}

// TestIssueViewTimelineJSON checks that the raw timeline reaches --json, which
// is how an agent would consume the commit SHA.
func TestIssueViewTimelineJSON(t *testing.T) {
	index := commitReferencing(t, "Timeline JSON target")
	num := strconv.FormatInt(index, 10)

	viewUntil(t, "referenced this issue from a commit",
		"issue", "view", num, "-R", adminUser+"/test-repo")

	stdout := mustRunFJ(t, "issue", "view", num, "-R", adminUser+"/test-repo", "--json")

	var result struct {
		Timeline []struct {
			Type         string `json:"type"`
			RefCommitSHA string `json:"ref_commit_sha"`
		} `json:"timeline"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}

	var found bool
	for _, e := range result.Timeline {
		if e.Type == "commit_ref" {
			found = true
			if len(e.RefCommitSHA) != 40 {
				t.Errorf("ref_commit_sha = %q, want a full SHA", e.RefCommitSHA)
			}
		}
	}
	if !found {
		t.Errorf("expected a commit_ref event in the JSON timeline:\n%s", stdout)
	}

	// --show-timeline=false must drop the key rather than emit an empty one.
	off := mustRunFJ(t, "issue", "view", num, "-R", adminUser+"/test-repo", "--json", "--show-timeline=false")
	var raw map[string]any
	if err := json.Unmarshal([]byte(off), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := raw["timeline"]; ok {
		t.Errorf("--show-timeline=false should omit the timeline key:\n%s", off)
	}
}

// TestIssueViewTimelineClose covers an event produced through fj itself, and
// checks the state-change annotation on the resulting reference.
func TestIssueViewTimelineClose(t *testing.T) {
	issue, _, err := testClient.CreateIssue(adminUser, "test-repo", forgejo.CreateIssueOption{
		Title: "Timeline close target",
	})
	if err != nil {
		t.Fatalf("creating issue: %v", err)
	}
	num := strconv.FormatInt(issue.Index, 10)

	mustRunFJ(t, "issue", "close", num, "-R", adminUser+"/test-repo")

	stdout := viewUntil(t, "closed this issue", "issue", "view", num, "-R", adminUser+"/test-repo")
	if !strings.Contains(stdout, adminUser+" closed this issue") {
		t.Errorf("expected %q to close the issue in the timeline:\n%s", adminUser, stdout)
	}
}

// TestIssueViewTimelineRefSources checks that a reference names where it came
// from. A bare "referenced this issue" is what the timeline looked like before
// ref_issue was read, and it is useless without the source. Each way of
// referencing an issue is exercised against a real Forgejo, since the event
// type alone does not reliably tell you the source's kind.
func TestIssueViewTimelineRefSources(t *testing.T) {
	owner, repo := adminUser, "test-repo"

	target, _, err := testClient.CreateIssue(owner, repo, forgejo.CreateIssueOption{
		Title: "Ref source target",
	})
	if err != nil {
		t.Fatalf("creating target issue: %v", err)
	}

	// A pull request whose body references the target.
	branch := fmt.Sprintf("ref-source-%d", target.Index)
	if _, _, err := testClient.CreateBranch(owner, repo, forgejo.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: "main",
	}); err != nil {
		t.Fatalf("creating branch: %v", err)
	}
	if _, _, err := testClient.CreateFile(owner, repo, branch+".txt", forgejo.CreateFileOptions{
		FileOptions: forgejo.FileOptions{Message: "ref source file", BranchName: branch},
		Content:     base64.StdEncoding.EncodeToString([]byte("probe\n")),
	}); err != nil {
		t.Fatalf("creating file on branch: %v", err)
	}
	pull, _, err := testClient.CreatePullRequest(owner, repo, forgejo.CreatePullRequestOption{
		Head:  branch,
		Base:  "main",
		Title: "Ref source pull request",
		Body:  fmt.Sprintf("This pull request body references #%d", target.Index),
	})
	if err != nil {
		t.Fatalf("creating pull request: %v", err)
	}

	// An issue whose body references the target, then a comment doing the same.
	src, _, err := testClient.CreateIssue(owner, repo, forgejo.CreateIssueOption{
		Title: "Ref source issue",
		Body:  fmt.Sprintf("This issue body references #%d", target.Index),
	})
	if err != nil {
		t.Fatalf("creating source issue: %v", err)
	}
	if _, _, err := testClient.CreateIssueComment(owner, repo, src.Index, forgejo.CreateIssueCommentOption{
		Body: fmt.Sprintf("This comment references #%d", target.Index),
	}); err != nil {
		t.Fatalf("creating comment: %v", err)
	}

	num := strconv.FormatInt(target.Index, 10)
	wantPull := fmt.Sprintf(`referenced this issue from pull request #%d "Ref source pull request"`, pull.Index)
	stdout := viewUntil(t, wantPull, "issue", "view", num, "-R", owner+"/"+repo)

	for _, want := range []string{
		wantPull,
		fmt.Sprintf(`referenced this issue from issue #%d "Ref source issue"`, src.Index),
		fmt.Sprintf(`referenced this issue from a comment on issue #%d "Ref source issue"`, src.Index),
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}
}

// TestPRViewTimeline checks the same default holds for pull requests, whose
// events say "pull request" rather than "issue".
func TestPRViewTimeline(t *testing.T) {
	prs, _, err := testClient.ListRepoPullRequests(adminUser, "test-repo", forgejo.ListPullRequestsOptions{})
	if err != nil {
		t.Fatalf("listing pull requests: %v", err)
	}
	if len(prs) == 0 {
		t.Skip("no seeded pull request to exercise")
	}
	pr := prs[0]
	num := strconv.FormatInt(pr.Index, 10)

	// A label change is an event fj renders, and needs no push to produce.
	mustRunFJ(t, "pr", "edit", num, "-R", adminUser+"/test-repo", "--add-label", "bug")

	stdout := viewUntil(t, "--- Timeline", "pr", "view", num, "-R", adminUser+"/test-repo")
	if !strings.Contains(stdout, "added the bug label") {
		t.Errorf("expected the label event in the pull request timeline:\n%s", stdout)
	}

	off := mustRunFJ(t, "pr", "view", num, "-R", adminUser+"/test-repo", "--show-timeline=false")
	if strings.Contains(off, "--- Timeline") {
		t.Errorf("--show-timeline=false should suppress the timeline:\n%s", off)
	}
}
