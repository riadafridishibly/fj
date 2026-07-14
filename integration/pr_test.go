//go:build integration

package integration

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
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
	stdout := mustRunFJ(t, "pr", "list", "-R", adminUser+"/test-repo", "--json")

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
	stdout := mustRunFJ(t, "pr", "view", "4", "-R", adminUser+"/test-repo", "--json")

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	pr, ok := result["pull_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'pull_request' key, got: %v", result)
	}
	if title, _ := pr["title"].(string); title != "Add feature-1" {
		t.Errorf("expected title 'Add feature-1', got %q", title)
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

	listOut := mustRunFJ(t, "pr", "review", "comment", "list", "5", repoFlag, repoName, "--json")
	var comments []struct {
		ID       int64  `json:"id"`
		Body     string `json:"body"`
		ReviewID int64  `json:"review_id"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal([]byte(listOut), &comments); err != nil {
		t.Fatalf("invalid JSON from comment list: %v\nraw: %s", err, listOut)
	}
	var original *struct {
		ID       int64  `json:"id"`
		Body     string `json:"body"`
		ReviewID int64  `json:"review_id"`
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
		"--body", "reply to inline comment", "--json")
	var reply struct {
		ID       int64  `json:"id"`
		Body     string `json:"body"`
		ReviewID int64  `json:"review_id"`
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
	listOut = mustRunFJ(t, "pr", "review", "comment", "list", "5", repoFlag, repoName, "--json")
	if !strings.Contains(listOut, "reply to inline comment") {
		t.Fatalf("reply not visible in comment list:\n%s", listOut)
	}

	// Delete the reply. --yes is required to delete; the comment details are
	// fetched and printed before the deletion happens.
	mustRunFJ(t, "pr", "review", "comment", "delete", "5",
		strconv.FormatInt(reply.ID, 10), repoFlag, repoName, "--yes")

	listOut = mustRunFJ(t, "pr", "review", "comment", "list", "5", repoFlag, repoName, "--json")
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
	out := mustRunFJ(t, "pr", "review", "comment", "list", pr, "-R", repo, "--json")
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
	out := mustRunFJ(t, "pr", "review", "comment", "list", pr, "-R", repo, "--json")
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
