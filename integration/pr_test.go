//go:build integration

package integration

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestPRList(t *testing.T) {
	stdout := mustRunFJ(t, "pr", "list", "-R", adminUser+"/test-repo")

	if !strings.Contains(stdout, "Add feature-1") {
		t.Errorf("expected 'Add feature-1' in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Add feature-2") {
		t.Errorf("expected 'Add feature-2' in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "feature-1") {
		t.Errorf("expected branch name 'feature-1' in output:\n%s", stdout)
	}
}

func TestPRListJSON(t *testing.T) {
	stdout := mustRunFJ(t, "pr", "list", "-R", adminUser+"/test-repo", "--json", "number")

	var prs []map[string]any
	if err := json.Unmarshal([]byte(stdout), &prs); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	if len(prs) < 2 {
		t.Errorf("expected at least 2 open PRs, got %d", len(prs))
	}
}

func TestPRView(t *testing.T) {
	// PRs are #4 and #5 (after 3 issues)
	stdout := mustRunFJ(t, "pr", "view", "4", "-R", adminUser+"/test-repo")

	for _, want := range []string{"Add feature-1", "#4", "open", "feature-1", "main"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}
}

func TestPRViewJSON(t *testing.T) {
	stdout := mustRunFJ(t, "pr", "view", "4", "-R", adminUser+"/test-repo", "--json", "title")

	var pr map[string]any
	if err := json.Unmarshal([]byte(stdout), &pr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if title, _ := pr["title"].(string); title != "Add feature-1" {
		t.Errorf("expected title 'Add feature-1', got %q", title)
	}
}

// TestPRReady converts a draft to ready and back. Forgejo has no draft flag,
// so both directions edit the title's work-in-progress prefix.
func TestPRReady(t *testing.T) {
	repo := adminUser + "/test-repo"
	if _, _, err := testClient.CreateFile(adminUser, "test-repo", "ready.txt", forgejo.CreateFileOptions{
		FileOptions: forgejo.FileOptions{Message: "Add ready.txt", NewBranchName: "pr-ready"},
		Content:     base64.StdEncoding.EncodeToString([]byte("ready\n")),
	}); err != nil {
		t.Fatalf("committing on pr-ready: %v", err)
	}
	out := mustRunFJ(t, "pr", "create", "-R", repo, "--title", "Ready test", "--head", "pr-ready",
		"--base", "main", "--draft", "--json", "number")
	var created struct {
		Number int64 `json:"number"`
	}
	if err := json.Unmarshal([]byte(out), &created); err != nil || created.Number == 0 {
		t.Fatalf("create = %s (%v), want a number", out, err)
	}
	num := strconv.FormatInt(created.Number, 10)
	// Closed afterwards so later tests that pick an open PR do not get this one.
	t.Cleanup(func() {
		closed := forgejo.StateClosed
		_, _, _ = testClient.EditPullRequest(adminUser, "test-repo", created.Number, forgejo.EditPullRequestOption{State: &closed})
	})

	check := func(wantTitle string, wantDraft bool) {
		t.Helper()
		out := mustRunFJ(t, "pr", "view", num, "-R", repo, "--json", "title,isDraft")
		var got struct {
			Title   string `json:"title"`
			IsDraft bool   `json:"isDraft"`
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil || got.Title != wantTitle || got.IsDraft != wantDraft {
			t.Fatalf("view = %s (%v), want title %q and isDraft %v", out, err, wantTitle, wantDraft)
		}
	}
	check("WIP: Ready test", true)

	// --draft lists only drafts and --draft=false all but drafts, as in gh.
	for flag, want := range map[string]bool{"--draft": true, "--draft=false": false} {
		out := mustRunFJ(t, "pr", "list", "-R", repo, flag, "--json", "number,isDraft")
		var prs []struct {
			Number  int64 `json:"number"`
			IsDraft bool  `json:"isDraft"`
		}
		if err := json.Unmarshal([]byte(out), &prs); err != nil {
			t.Fatalf("pr list %s: invalid JSON: %v\n%s", flag, err, out)
		}
		listed := false
		for _, pr := range prs {
			listed = listed || pr.Number == created.Number
			if pr.IsDraft != want {
				t.Errorf("pr list %s listed #%d with isDraft %v", flag, pr.Number, pr.IsDraft)
			}
		}
		if listed != want {
			t.Errorf("pr list %s: draft #%d listed = %v, want %v", flag, created.Number, listed, want)
		}
	}

	mustRunFJ(t, "pr", "ready", num, "-R", repo)
	check("Ready test", false)

	_, stderr, err := runFJ("pr", "ready", num, "-R", repo)
	if err != nil || !strings.Contains(stderr, `is already "ready for review"`) {
		t.Errorf("ready on a ready PR: err = %v, stderr = %q; want exit 0 and already ready", err, stderr)
	}

	mustRunFJ(t, "pr", "ready", num, "-R", repo, "--undo")
	check("WIP: Ready test", true)

	mustRunFJ(t, "pr", "edit", num, "-R", repo, "--title", "[wip] Ready test")
	check("[wip] Ready test", true)
	mustRunFJ(t, "pr", "ready", num, "-R", repo)
	check("Ready test", false)

	// A title that is only a prefix would be empty, which Forgejo ignores.
	mustRunFJ(t, "pr", "edit", num, "-R", repo, "--title", "WIP:")
	_, stderr, err = runFJ("pr", "ready", num, "-R", repo)
	if err == nil || !strings.Contains(stderr, "has no title besides") {
		t.Errorf("ready on a title that is only WIP:: err = %v, stderr = %q; want an error", err, stderr)
	}

	mustRunFJ(t, "pr", "close", num, "-R", repo)
	_, stderr, err = runFJ("pr", "ready", num, "-R", repo, "--undo")
	if err == nil || !strings.Contains(stderr, "is closed") {
		t.Errorf("ready on a closed PR: err = %v, stderr = %q; want an error", err, stderr)
	}
}

// TestPRReviewCommentLifecycle exercises the inline review comment flow
// end to end: create a review with an inline comment, list it, reply to
// it (same review/path/line via the raw API wrapper), then delete the
// reply and verify it is gone.
func TestPRReviewCommentLifecycle(t *testing.T) {
	repoFlag := "-R"
	repoName := adminUser + "/test-repo"

	// PR #5 is feature-2; its diff adds feature-2.txt with one line.
	mustRunFJ(t, "pr", "review", "create", "5", repoFlag, repoName,
		"--comment",
		"--comment-path", "feature-2.txt",
		"--comment-line", "1",
		"--comment-body", "original inline comment")

	listOut := mustRunFJ(t, "pr", "review", "comment", "list", "5", repoFlag, repoName, "--json", "id,body,reviewId,path")
	var comments []struct {
		ID       int64  `json:"id"`
		Body     string `json:"body"`
		ReviewID int64  `json:"reviewId"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal([]byte(listOut), &comments); err != nil {
		t.Fatalf("invalid JSON from comment list: %v\nraw: %s", err, listOut)
	}
	var original *struct {
		ID       int64  `json:"id"`
		Body     string `json:"body"`
		ReviewID int64  `json:"reviewId"`
		Path     string `json:"path"`
	}
	for i := range comments {
		if comments[i].Body == "original inline comment" {
			original = &comments[i]
		}
	}
	if original == nil {
		t.Fatalf("original inline comment not found in list output:\n%s", listOut)
	}

	replyOut := mustRunFJ(t, "pr", "review", "comment", "reply", "5",
		strconv.FormatInt(original.ID, 10), repoFlag, repoName,
		"--body", "reply to inline comment", "--json", "id,body,reviewId,path")
	var reply struct {
		ID       int64  `json:"id"`
		Body     string `json:"body"`
		ReviewID int64  `json:"reviewId"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal([]byte(replyOut), &reply); err != nil {
		t.Fatalf("invalid JSON from reply: %v\nraw: %s", err, replyOut)
	}
	if reply.ID == 0 || reply.Body != "reply to inline comment" {
		t.Fatalf("unexpected reply payload: %+v", reply)
	}
	if reply.ReviewID != original.ReviewID {
		t.Errorf("reply landed on review %d, want %d (same thread)", reply.ReviewID, original.ReviewID)
	}
	if reply.Path != original.Path {
		t.Errorf("reply path %q, want %q", reply.Path, original.Path)
	}

	// The reply must be visible when listing again.
	listOut = mustRunFJ(t, "pr", "review", "comment", "list", "5", repoFlag, repoName, "--json", "body")
	if !strings.Contains(listOut, "reply to inline comment") {
		t.Fatalf("reply not visible in comment list:\n%s", listOut)
	}

	// Delete the reply. --yes is required to delete; the comment details are
	// fetched and printed before the deletion happens.
	mustRunFJ(t, "pr", "review", "comment", "delete", "5",
		strconv.FormatInt(reply.ID, 10), repoFlag, repoName, "--yes")

	listOut = mustRunFJ(t, "pr", "review", "comment", "list", "5", repoFlag, repoName, "--json", "body")
	if strings.Contains(listOut, "reply to inline comment") {
		t.Errorf("deleted reply still present in comment list:\n%s", listOut)
	}
	if !strings.Contains(listOut, "original inline comment") {
		t.Errorf("original comment unexpectedly missing after deleting reply:\n%s", listOut)
	}

	// Deleting a nonexistent comment must fail loudly, not silently no-op.
	if _, _, err := runFJ("pr", "review", "comment", "delete", "5", "999999", repoFlag, repoName, "--yes"); err == nil {
		t.Errorf("deleting nonexistent comment should fail")
	}
}

// prReviewCommentIDByBody finds the ID of the first inline review comment whose
// body matches, via the JSON review comment list for the given PR.
func prReviewCommentIDByBody(t *testing.T, pr, repo, body string) int64 {
	t.Helper()
	out := mustRunFJ(t, "pr", "review", "comment", "list", pr, "-R", repo, "--json", "id,body")
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(out), &comments); err != nil {
		t.Fatalf("invalid JSON from review comment list: %v\nraw: %s", err, out)
	}
	for _, c := range comments {
		if c.Body == body {
			return c.ID
		}
	}
	return 0
}

// prReviewCommentExists reports whether an inline review comment with the given
// ID is still present.
func prReviewCommentExists(t *testing.T, pr, repo string, id int64) bool {
	t.Helper()
	out := mustRunFJ(t, "pr", "review", "comment", "list", pr, "-R", repo, "--json", "id")
	var comments []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &comments); err != nil {
		t.Fatalf("invalid JSON from review comment list: %v\nraw: %s", err, out)
	}
	for _, c := range comments {
		if c.ID == id {
			return true
		}
	}
	return false
}

// TestPRReviewCommentDelete covers the --yes/--dry-run contract for deleting an
// inline review comment, mirroring TestMilestoneDelete / TestReleaseDelete.
func TestPRReviewCommentDelete(t *testing.T) {
	repoFlag := "-R"
	repoName := adminUser + "/test-repo"
	pr := "5"
	body := "review-comment-delete-contract"

	// PR #5 (feature-2) adds feature-2.txt with a single line; comment on it.
	mustRunFJ(t, "pr", "review", "create", pr, repoFlag, repoName,
		"--comment", "--comment-path", "feature-2.txt", "--comment-line", "1",
		"--comment-body", body)

	id := prReviewCommentIDByBody(t, pr, repoName, body)
	if id == 0 {
		t.Fatal("could not find created review comment id")
	}

	// Without --yes or --dry-run, the command must refuse to delete.
	if _, _, err := runFJ("pr", "review", "comment", "delete", pr, strconv.FormatInt(id, 10), repoFlag, repoName); err == nil {
		t.Fatalf("pr review comment delete without --yes should fail")
	}

	// --dry-run must resolve and print the comment but leave it intact.
	stdout, stderr, err := runFJ("pr", "review", "comment", "delete", pr, strconv.FormatInt(id, 10), repoFlag, repoName, "--dry-run")
	if err != nil {
		t.Fatalf("pr review comment delete --dry-run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, body) {
		t.Errorf("dry-run output should mention the comment body\nstderr: %s", stderr)
	}
	if !strings.Contains(stderr, "dry-run") {
		t.Errorf("dry-run output should state nothing was changed\nstderr: %s", stderr)
	}
	if !prReviewCommentExists(t, pr, repoName, id) {
		t.Errorf("dry-run must not delete the review comment")
	}

	// --yes performs the actual deletion.
	_, stderr, err = runFJ("pr", "review", "comment", "delete", pr, strconv.FormatInt(id, 10), repoFlag, repoName, "--yes")
	if err != nil {
		t.Fatalf("pr review comment delete failed: %v\nstderr: %s", err, stderr)
	}
	if prReviewCommentExists(t, pr, repoName, id) {
		t.Errorf("expected review comment #%d to be deleted", id)
	}
}
