//go:build integration

package integration

import (
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// gitRepoOnBranch creates a throwaway repository whose only remote points
// at the test server and whose HEAD is branch. Nothing is fetched: repo
// resolution reads the remote URL and the branch name reads from HEAD, so
// an unborn branch is enough.
func gitRepoOnBranch(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	remote := forgejoURL + "/" + adminUser + "/test-repo.git"
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"remote", "add", "origin", remote},
		{"checkout", "--quiet", "-b", branch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// TestPRCurrentBranchDefault covers tier 0.2: PR commands resolve the pull
// request for the checked-out branch when the number is omitted.
func TestPRCurrentBranchDefault(t *testing.T) {
	dir := gitRepoOnBranch(t, "feature-1") // PR #4

	stdout, stderr, err := runFJIn(dir, "pr", "view")
	if err != nil {
		t.Fatalf("fj pr view failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	for _, want := range []string{"Add feature-1", "#4"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}

	diff, stderr, err := runFJIn(dir, "pr", "diff")
	if err != nil {
		t.Fatalf("fj pr diff failed: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(diff, "feature-1.txt") {
		t.Errorf("expected feature-1.txt in diff:\n%s", diff)
	}
}

func TestPRCurrentBranchNoPullRequest(t *testing.T) {
	dir := gitRepoOnBranch(t, "branch-without-a-pr")

	stdout, stderr, err := runFJIn(dir, "pr", "view")
	if err == nil {
		t.Fatalf("expected failure for a branch with no PR, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "no open pull request found") {
		t.Errorf("expected a clear no-PR error, got: %s", stderr)
	}
}

// TestCommentGroupDispatch covers tier 0.1: the comment group accepts
// gh's leaf-verb spelling while keeping its subcommands.
func TestCommentGroupDispatch(t *testing.T) {
	repo := adminUser + "/test-repo"

	mustRunFJ(t, "issue", "comment", "1", "-R", repo, "--body", "group dispatch on issue")
	listOut := mustRunFJ(t, "issue", "comment", "list", "1", "-R", repo)
	if !strings.Contains(listOut, "group dispatch on issue") {
		t.Errorf("comment not found after group dispatch:\n%s", listOut)
	}

	mustRunFJ(t, "pr", "comment", "4", "-R", repo, "--body", "group dispatch on pr")
	prList := mustRunFJ(t, "pr", "comment", "list", "4", "-R", repo)
	if !strings.Contains(prList, "group dispatch on pr") {
		t.Errorf("comment not found after group dispatch:\n%s", prList)
	}
}

func TestCommentGroupDispatchCurrentBranch(t *testing.T) {
	dir := gitRepoOnBranch(t, "feature-1") // PR #4

	stdout, stderr, err := runFJIn(dir, "pr", "comment", "--body", "dispatch plus branch default")
	if err != nil {
		t.Fatalf("fj pr comment failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	listOut := mustRunFJ(t, "pr", "comment", "list", "4", "-R", adminUser+"/test-repo")
	if !strings.Contains(listOut, "dispatch plus branch default") {
		t.Errorf("comment not found on PR #4:\n%s", listOut)
	}
}

// TestCommentGroupHelp keeps the bare group printing help rather than
// failing on a missing body.
func TestCommentGroupHelp(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "comment")
	for _, want := range []string{"Available Commands:", "create", "--body"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in group help:\n%s", want, stdout)
		}
	}
}

// TestReviewGroupDispatch covers tier 0.1 for `fj pr review <n> [flags]`.
func TestReviewGroupDispatch(t *testing.T) {
	repo := adminUser + "/test-repo"

	mustRunFJ(t, "pr", "review", "5", "-R", repo, "--comment", "--body", "review via group dispatch")

	// The review list table has no body column, so match on the payload.
	type review struct {
		Body string `json:"body"`
	}
	listOut := mustRunFJ(t, "pr", "review", "list", "5", "-R", repo, "--json")
	var reviews []review
	if err := json.Unmarshal([]byte(listOut), &reviews); err != nil {
		t.Fatalf("invalid JSON from review list: %v\nraw: %s", err, listOut)
	}
	if !slices.ContainsFunc(reviews, func(r review) bool { return r.Body == "review via group dispatch" }) {
		t.Errorf("review not found after group dispatch: %+v", reviews)
	}

	help := mustRunFJ(t, "pr", "review")
	if !strings.Contains(help, "Available Commands:") {
		t.Errorf("expected help from the bare review group:\n%s", help)
	}
}

// TestRefFromURL covers tier 0.4: a web URL is a reference, as it is for
// every gh command that takes an issue or pull request.
func TestRefFromURL(t *testing.T) {
	issueURL := forgejoURL + "/" + adminUser + "/test-repo/issues/1"
	stdout := mustRunFJ(t, "issue", "view", issueURL)
	if !strings.Contains(stdout, "First issue") {
		t.Errorf("expected issue #1 from its URL:\n%s", stdout)
	}

	prURL := forgejoURL + "/" + adminUser + "/test-repo/pulls/4"
	stdout = mustRunFJ(t, "pr", "view", prURL)
	if !strings.Contains(stdout, "Add feature-1") {
		t.Errorf("expected pr #4 from its URL:\n%s", stdout)
	}
}

// TestRefFromBranchArgument covers the branch form gh accepts on pull
// request commands. The command runs on feature-2 to prove the argument
// wins over the checked-out branch.
func TestRefFromBranchArgument(t *testing.T) {
	dir := gitRepoOnBranch(t, "feature-2") // PR #5

	stdout, stderr, err := runFJIn(dir, "pr", "view", "feature-1") // PR #4
	if err != nil {
		t.Fatalf("fj pr view feature-1 failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	for _, want := range []string{"Add feature-1", "#4"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}
}

// TestRefWithRepository covers fj's OWNER/REPO#N form, whose point is
// addressing a repository from anywhere: this runs with no git remote at
// all and without -R.
func TestRefWithRepository(t *testing.T) {
	stdout := mustRunFJ(t, "issue", "view", adminUser+"/test-repo#1")
	if !strings.Contains(stdout, "First issue") {
		t.Errorf("expected issue #1 from OWNER/REPO#N:\n%s", stdout)
	}
}

// TestRefMalformed keeps a mistyped URL a usage error rather than a
// branch lookup or a 404 against the wrong repository.
func TestRefMalformed(t *testing.T) {
	stdout, stderr, err := runFJ("issue", "view", "https://example.com/not-an-issue",
		"-R", adminUser+"/test-repo")
	if err == nil {
		t.Fatalf("expected failure for a malformed reference, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "invalid reference") {
		t.Errorf("expected a usage error naming the accepted forms, got: %s", stderr)
	}
}

// TestRepoOverrideWithHost covers tier 0.3: -R takes [HOST/]OWNER/REPO and
// rejects anything else instead of silently splitting it wrong.
func TestRepoOverrideWithHost(t *testing.T) {
	stdout := mustRunFJ(t, "repo", "view", "-R", forgejoHost+"/"+adminUser+"/test-repo")
	if !strings.Contains(stdout, adminUser+"/test-repo") {
		t.Errorf("expected the repo full name in output:\n%s", stdout)
	}

	stdout, stderr, err := runFJ("issue", "list", "-R", forgejoHost+"/"+adminUser+"/test-repo/extra")
	if err == nil {
		t.Fatalf("expected failure for a four-segment selector, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "[HOST/]OWNER/REPO") {
		t.Errorf("expected a format error naming the accepted shape, got: %s", stderr)
	}
}
