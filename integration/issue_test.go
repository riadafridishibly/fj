//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestIssueList(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo")

	if !strings.Contains(stdout, "First issue") {
		t.Errorf("expected 'First issue' in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Second issue") {
		t.Errorf("expected 'Second issue' in output:\n%s", stdout)
	}
	// Closed issue should NOT appear in default (open) listing
	if strings.Contains(stdout, "Closed issue") {
		t.Errorf("closed issue should not appear in open listing:\n%s", stdout)
	}
}

func TestIssueListClosed(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "--state", "closed")

	if !strings.Contains(stdout, "Closed issue") {
		t.Errorf("expected 'Closed issue' in output:\n%s", stdout)
	}
}

func TestIssueListAll(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "--state", "all")

	for _, want := range []string{"First issue", "Second issue", "Closed issue"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}
}

func TestIssueListByLabel(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "--label", "bug")

	if !strings.Contains(stdout, "First issue") {
		t.Errorf("expected 'First issue' (bug label) in output:\n%s", stdout)
	}
	if strings.Contains(stdout, "Second issue") {
		t.Errorf("'Second issue' (enhancement) should not appear in bug filter:\n%s", stdout)
	}
}

func TestIssueListJSON(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "--json")

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout), &issues); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	if len(issues) < 2 {
		t.Errorf("expected at least 2 open issues, got %d", len(issues))
	}
}

func TestIssueView(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "view", "1", "-R", adminUser+"/test-repo")

	for _, want := range []string{"First issue", "#1", "open", "bug"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}
}

func TestIssueViewJSON(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "view", "1", "-R", adminUser+"/test-repo", "--json")

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	issue, ok := result["issue"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'issue' key in response, got: %v", result)
	}
	if title, _ := issue["title"].(string); title != "First issue" {
		t.Errorf("expected title 'First issue', got %q", title)
	}
}

func TestIssueCreate(t *testing.T) {
	stdout, stderr, err := runFJ("issue", "create",
		"-R", adminUser+"/test-repo",
		"--title", "Created by test",
		"--body", "This issue was created by the integration test")
	if err != nil {
		t.Fatalf("issue create failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Output should contain the issue URL
	if !strings.Contains(stdout, "test-repo") {
		t.Errorf("expected issue URL in output:\n%s", stdout)
	}

	// Verify it shows up in the list
	listOut := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo")
	if !strings.Contains(listOut, "Created by test") {
		t.Errorf("newly created issue not found in list:\n%s", listOut)
	}
}

func TestIssueClose(t *testing.T) {
	// Create an issue to close
	runFJ("issue", "create",
		"-R", adminUser+"/test-repo",
		"--title", "Issue to close")

	// Find its number from the JSON list
	listOut := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "--json")
	var issues []map[string]any
	if err := json.Unmarshal([]byte(listOut), &issues); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	var issueNum string
	for _, iss := range issues {
		if title, _ := iss["title"].(string); title == "Issue to close" {
			if num, ok := iss["number"].(float64); ok {
				issueNum = fmt.Sprintf("%d", int(num))
			}
			break
		}
	}
	if issueNum == "" {
		t.Fatal("could not find 'Issue to close' in issue list")
	}

	// Close it
	_, stderr, err := runFJ("issue", "close", issueNum, "-R", adminUser+"/test-repo")
	if err != nil {
		t.Fatalf("issue close failed: %v\nstderr: %s", err, stderr)
	}

	// Verify it's closed
	viewOut := mustRunFJ(t, "issue", "view", issueNum, "-R", adminUser+"/test-repo")
	if !strings.Contains(viewOut, "closed") {
		t.Errorf("expected issue to be closed:\n%s", viewOut)
	}
}

// issueNumberByTitle finds the issue number for the first issue whose title
// matches, via the JSON issue list.
func issueNumberByTitle(t *testing.T, repo, title string) string {
	t.Helper()
	out := mustRunFJ(t, "issue", "list", "-R", repo, "--json")
	var issues []map[string]any
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, iss := range issues {
		if got, _ := iss["title"].(string); got == title {
			if num, ok := iss["number"].(float64); ok {
				return strconv.Itoa(int(num))
			}
		}
	}
	return ""
}

// commentIDByBody finds the ID of the first issue comment whose body matches,
// via the JSON comment list for the given issue.
func commentIDByBody(t *testing.T, issue, repo, body string) int64 {
	t.Helper()
	out := mustRunFJ(t, "issue", "comment", "list", issue, "-R", repo, "--json")
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(out), &comments); err != nil {
		t.Fatalf("invalid JSON from comment list: %v\nraw: %s", err, out)
	}
	for _, c := range comments {
		if c.Body == body {
			return c.ID
		}
	}
	return 0
}

func TestIssueDelete(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "issue-delete-contract"

	if _, stderr, err := runFJ("issue", "create", "-R", repo, "--title", title); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}
	num := issueNumberByTitle(t, repo, title)
	if num == "" {
		t.Fatal("could not find created issue number")
	}
	idx, _ := strconv.ParseInt(num, 10, 64)

	// Without --yes or --dry-run, the command must refuse to delete.
	if _, _, err := runFJ("issue", "delete", num, "-R", repo); err == nil {
		t.Fatalf("issue delete without --yes should fail")
	}

	// --dry-run must resolve and print the issue but leave it intact.
	stdout, stderr, err := runFJ("issue", "delete", num, "-R", repo, "--dry-run")
	if err != nil {
		t.Fatalf("issue delete --dry-run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, title) {
		t.Errorf("dry-run output should mention the issue title\nstderr: %s", stderr)
	}
	if !strings.Contains(stderr, "dry-run") {
		t.Errorf("dry-run output should state nothing was changed\nstderr: %s", stderr)
	}
	if _, _, err := testClient.GetIssue(adminUser, "test-repo", idx); err != nil {
		t.Errorf("dry-run must not delete the issue: %v", err)
	}

	// --yes performs the actual deletion.
	_, stderr, err = runFJ("issue", "delete", num, "-R", repo, "--yes")
	if err != nil {
		t.Fatalf("issue delete failed: %v\nstderr: %s", err, stderr)
	}
	if _, _, err := testClient.GetIssue(adminUser, "test-repo", idx); err == nil {
		t.Errorf("expected issue #%s to be deleted", num)
	}
}

func TestIssueCommentDelete(t *testing.T) {
	repo := adminUser + "/test-repo"
	issueTitle := "issue-for-comment-delete"

	if _, stderr, err := runFJ("issue", "create", "-R", repo, "--title", issueTitle); err != nil {
		t.Fatalf("setup create issue failed: %v\n%s", err, stderr)
	}
	issueNum := issueNumberByTitle(t, repo, issueTitle)
	if issueNum == "" {
		t.Fatal("could not find issue to comment on")
	}

	body := "comment-delete-contract"
	if _, stderr, err := runFJ("issue", "comment", "create", issueNum, "-R", repo, "--body", body); err != nil {
		t.Fatalf("setup create comment failed: %v\n%s", err, stderr)
	}
	id := commentIDByBody(t, issueNum, repo, body)
	if id == 0 {
		t.Fatal("could not find created comment id")
	}

	// Without --yes or --dry-run, the command must refuse to delete.
	if _, _, err := runFJ("issue", "comment", "delete", strconv.FormatInt(id, 10), "-R", repo); err == nil {
		t.Fatalf("issue comment delete without --yes should fail")
	}

	// --dry-run must resolve and print the comment but leave it intact.
	stdout, stderr, err := runFJ("issue", "comment", "delete", strconv.FormatInt(id, 10), "-R", repo, "--dry-run")
	if err != nil {
		t.Fatalf("issue comment delete --dry-run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, body) {
		t.Errorf("dry-run output should mention the comment body\nstderr: %s", stderr)
	}
	if !strings.Contains(stderr, "dry-run") {
		t.Errorf("dry-run output should state nothing was changed\nstderr: %s", stderr)
	}
	if _, _, err := testClient.GetIssueComment(adminUser, "test-repo", id); err != nil {
		t.Errorf("dry-run must not delete the comment: %v", err)
	}

	// --yes performs the actual deletion.
	_, stderr, err = runFJ("issue", "comment", "delete", strconv.FormatInt(id, 10), "-R", repo, "--yes")
	if err != nil {
		t.Fatalf("issue comment delete failed: %v\nstderr: %s", err, stderr)
	}
	if _, _, err := testClient.GetIssueComment(adminUser, "test-repo", id); err == nil {
		t.Errorf("expected comment #%d to be deleted", id)
	}
}
