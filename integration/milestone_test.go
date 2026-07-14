//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// TestMilestoneCreate creates a milestone via the CLI and verifies it via the SDK.
func TestMilestoneCreate(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-create"

	stdout, stderr, err := runFJ("milestone", "create",
		"-R", repo,
		"--title", title,
		"--description", "Created by integration test",
		"--due-date", "2030-06-15")
	if err != nil {
		t.Fatalf("milestone create failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "/milestone/") {
		t.Errorf("expected milestone URL in output:\n%s", stdout)
	}

	ms, _, err := testClient.GetMilestoneByName(adminUser, "test-repo", title)
	if err != nil {
		t.Fatalf("SDK GetMilestoneByName failed: %v", err)
	}
	if ms.Description != "Created by integration test" {
		t.Errorf("expected description to match, got %q", ms.Description)
	}
	if ms.Deadline == nil || ms.Deadline.UTC().Format("2006-01-02") != "2030-06-15" {
		t.Errorf("expected due date 2030-06-15, got %v", ms.Deadline)
	}
	if ms.State != forgejo.StateOpen {
		t.Errorf("expected milestone to be open, got %s", ms.State)
	}
}

// TestMilestoneList lists milestones and checks the created one appears.
func TestMilestoneList(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-list"

	if _, stderr, err := runFJ("milestone", "create", "-R", repo, "--title", title); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "milestone", "list", "-R", repo)
	if !strings.Contains(stdout, title) {
		t.Errorf("expected milestone %q in list output:\n%s", title, stdout)
	}
}

// TestMilestoneListJSON verifies --json output is valid and filters by state.
func TestMilestoneListJSON(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-list-json"

	if _, stderr, err := runFJ("milestone", "create", "-R", repo, "--title", title); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}
	if _, stderr, err := runFJ("milestone", "close", title, "-R", repo); err != nil {
		t.Fatalf("setup close failed: %v\n%s", err, stderr)
	}

	// Closed milestone must not appear in the default (open) list.
	stdout := mustRunFJ(t, "milestone", "list", "-R", repo, "--json")
	var open []map[string]any
	if err := json.Unmarshal([]byte(stdout), &open); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	for _, m := range open {
		if m["title"] == title {
			t.Errorf("closed milestone %q should not appear in open list", title)
		}
	}

	stdout = mustRunFJ(t, "milestone", "list", "-R", repo, "--state", "closed", "--json")
	var closed []map[string]any
	if err := json.Unmarshal([]byte(stdout), &closed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	var found bool
	for _, m := range closed {
		if m["title"] == title {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected milestone %q in closed list", title)
	}
}

// TestMilestoneView shows a milestone by title.
func TestMilestoneView(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-view"

	if _, stderr, err := runFJ("milestone", "create", "-R", repo,
		"--title", title,
		"--description", "View body content",
		"--due-date", "2031-01-31"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "milestone", "view", title, "-R", repo)
	for _, want := range []string{title, "State: open", "Due date: 2031-01-31", "View body content"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in view output:\n%s", want, stdout)
		}
	}
}

// TestMilestoneViewJSON verifies --json output for view.
func TestMilestoneViewJSON(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-view-json"

	if _, stderr, err := runFJ("milestone", "create", "-R", repo, "--title", title); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "milestone", "view", title, "-R", repo, "--json")
	var ms map[string]any
	if err := json.Unmarshal([]byte(stdout), &ms); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	if ms["title"] != title {
		t.Errorf("expected title %q, got %v", title, ms["title"])
	}
}

// TestMilestoneEdit renames a milestone and changes its description.
func TestMilestoneEdit(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-edit"
	renamed := "ms-edit-renamed"

	if _, stderr, err := runFJ("milestone", "create", "-R", repo,
		"--title", title, "--description", "original"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	_, stderr, err := runFJ("milestone", "edit", title, "-R", repo,
		"--title", renamed,
		"--description", "updated")
	if err != nil {
		t.Fatalf("milestone edit failed: %v\nstderr: %s", err, stderr)
	}

	ms, _, err := testClient.GetMilestoneByName(adminUser, "test-repo", renamed)
	if err != nil {
		t.Fatalf("SDK GetMilestoneByName failed: %v", err)
	}
	if ms.Description != "updated" {
		t.Errorf("expected description 'updated', got %q", ms.Description)
	}
}

// TestMilestoneEditRequiresFlag verifies edit fails without any change flag.
func TestMilestoneEditRequiresFlag(t *testing.T) {
	repo := adminUser + "/test-repo"

	if _, _, err := runFJ("milestone", "edit", "v1.0", "-R", repo); err == nil {
		t.Error("expected edit to fail without any change flags")
	}
}

// TestMilestoneCloseReopen closes and reopens a milestone.
func TestMilestoneCloseReopen(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-close-reopen"

	if _, stderr, err := runFJ("milestone", "create", "-R", repo, "--title", title); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	if _, stderr, err := runFJ("milestone", "close", title, "-R", repo); err != nil {
		t.Fatalf("milestone close failed: %v\nstderr: %s", err, stderr)
	}
	ms, _, err := testClient.GetMilestoneByName(adminUser, "test-repo", title)
	if err != nil {
		t.Fatalf("SDK GetMilestoneByName failed: %v", err)
	}
	if ms.State != forgejo.StateClosed {
		t.Errorf("expected state closed, got %s", ms.State)
	}

	if _, stderr, err := runFJ("milestone", "reopen", title, "-R", repo); err != nil {
		t.Fatalf("milestone reopen failed: %v\nstderr: %s", err, stderr)
	}
	ms, _, err = testClient.GetMilestoneByName(adminUser, "test-repo", title)
	if err != nil {
		t.Fatalf("SDK GetMilestoneByName failed: %v", err)
	}
	if ms.State != forgejo.StateOpen {
		t.Errorf("expected state open, got %s", ms.State)
	}
}

// TestMilestoneDelete deletes a milestone and verifies it's gone.
func TestMilestoneDelete(t *testing.T) {
	repo := adminUser + "/test-repo"
	title := "ms-delete"

	if _, stderr, err := runFJ("milestone", "create", "-R", repo, "--title", title); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	_, stderr, err := runFJ("milestone", "delete", title, "-R", repo, "--yes")
	if err != nil {
		t.Fatalf("milestone delete failed: %v\nstderr: %s", err, stderr)
	}

	if _, _, err := testClient.GetMilestoneByName(adminUser, "test-repo", title); err == nil {
		t.Errorf("expected milestone %q to be deleted", title)
	}
}
