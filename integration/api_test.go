//go:build integration

package integration

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// TestAPIPaginate walks the labels one per page. The container's Link
// headers name its ROOT_URL (localhost:3000) while the test reaches it on a
// mapped port, so this only passes if fj keeps every page on the host it
// first reached.
func TestAPIPaginate(t *testing.T) {
	repo := adminUser + "/test-repo"

	var all []map[string]any
	if err := json.Unmarshal([]byte(mustRunFJ(t, "api", "-R", repo, "repos/{owner}/{repo}/labels?limit=50")), &all); err != nil {
		t.Fatalf("single page is not a JSON array: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("got %d labels, need at least 2 for more than one page", len(all))
	}

	var paged []map[string]any
	out := mustRunFJ(t, "api", "-R", repo, "--paginate", "repos/{owner}/{repo}/labels?limit=1")
	if err := json.Unmarshal([]byte(out), &paged); err != nil {
		t.Fatalf("--paginate output is not one JSON array: %v\n%s", err, out)
	}
	if len(paged) != len(all) {
		t.Errorf("--paginate returned %d labels, want %d", len(paged), len(all))
	}

	names := mustRunFJ(t, "api", "-R", repo, "--paginate", "-X", "GET", "-F", "limit=1", "--jq", ".[].name", "repos/{owner}/{repo}/labels")
	if got := strings.Count(names, "\n"); got != len(all) {
		t.Errorf("--jq printed %d lines, want %d:\n%s", got, len(all), names)
	}
	if !strings.Contains(names, "bug\n") {
		t.Errorf("--jq output lacks the bug label:\n%s", names)
	}
}

// TestAPICreate posts fields as a JSON body and checks the label landed.
func TestAPICreate(t *testing.T) {
	repo := adminUser + "/test-repo"

	id := mustRunFJ(t, "api", "-R", repo, "repos/{owner}/{repo}/labels",
		"-f", "name=made-by-fj-api", "-f", "color=#123456", "-F", "exclusive=false", "--jq", ".id")
	id = strings.TrimSpace(id)

	got := mustRunFJ(t, "api", "-R", repo, "repos/{owner}/{repo}/labels/"+id, "--jq", ".name")
	if strings.TrimSpace(got) != "made-by-fj-api" {
		t.Errorf("label %s name = %q", id, got)
	}
}

// TestAPINotFound pins the error contract: the server's body on stdout,
// the message and status on stderr, exit 1.
func TestAPINotFound(t *testing.T) {
	stdout, stderr, err := runFJ("api", "-R", adminUser+"/test-repo", "repos/{owner}/{repo}/issues/99999", "--jq", ".title")

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit 1\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, `"message"`) {
		t.Errorf("stdout = %q, want the unfiltered error body", stdout)
	}
	if !strings.Contains(stderr, "(HTTP 404)") {
		t.Errorf("stderr = %q, want the status", stderr)
	}
}
