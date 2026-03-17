//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
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
