//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRepoList(t *testing.T) {
	stdout := mustRunFJ(t, "repo", "list")

	if !strings.Contains(stdout, "test-repo") {
		t.Errorf("expected 'test-repo' in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "another-repo") {
		t.Errorf("expected 'another-repo' in output:\n%s", stdout)
	}
}

func TestRepoListJSON(t *testing.T) {
	stdout := mustRunFJ(t, "repo", "list", "--json")

	var repos []map[string]any
	if err := json.Unmarshal([]byte(stdout), &repos); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	// Should have at least test-repo, another-repo, private-repo
	if len(repos) < 3 {
		t.Errorf("expected at least 3 repos, got %d", len(repos))
	}
}

func TestRepoListVisibility(t *testing.T) {
	stdout := mustRunFJ(t, "repo", "list", "--visibility", "private")

	if !strings.Contains(stdout, "private-repo") {
		t.Errorf("expected 'private-repo' in output:\n%s", stdout)
	}
	// Public repos should not appear
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "test-repo") && !strings.Contains(line, "private") {
			t.Errorf("unexpected public 'test-repo' in private listing:\n%s", stdout)
			break
		}
	}
}

func TestRepoView(t *testing.T) {
	stdout := mustRunFJ(t, "repo", "view", adminUser+"/test-repo")

	for _, want := range []string{"test-repo", "public", "main"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}
}

func TestRepoViewJSON(t *testing.T) {
	stdout := mustRunFJ(t, "repo", "view", adminUser+"/test-repo", "--json")

	var repo map[string]any
	if err := json.Unmarshal([]byte(stdout), &repo); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if repo["name"] != "test-repo" {
		t.Errorf("expected name 'test-repo', got %v", repo["name"])
	}
}

func TestRepoCreate(t *testing.T) {
	stdout, stderr, err := runFJ("repo", "create", "created-by-test", "--description", "Created by integration test")
	if err != nil {
		t.Fatalf("repo create failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Verify it exists
	viewOut := mustRunFJ(t, "repo", "view", adminUser+"/created-by-test", "--json")
	var repo map[string]any
	if err := json.Unmarshal([]byte(viewOut), &repo); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if repo["name"] != "created-by-test" {
		t.Errorf("expected name 'created-by-test', got %v", repo["name"])
	}
	if desc, ok := repo["description"].(string); !ok || desc != "Created by integration test" {
		t.Errorf("expected description 'Created by integration test', got %v", repo["description"])
	}
}
