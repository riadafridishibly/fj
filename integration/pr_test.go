//go:build integration

package integration

import (
	"encoding/json"
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
