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
