//go:build integration

package integration

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("fj did not run to completion: %v", err)
	}
	return ee.ExitCode()
}

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
	// The message prefix is the stable, greppable contract for scripts.
	if !strings.Contains(stderr, `no open pull request found for branch "branch-without-a-pr"`) {
		t.Errorf("expected the stable no-PR error, got: %s", stderr)
	}
	if code := exitCode(t, err); code != 1 {
		t.Errorf("no-PR-for-branch exit code = %d, want 1", code)
	}
}

// TestUsageErrorExitCodes pins missing/invalid-number usage errors at exit
// code 2 — the current contract. gh reserves 2 for "cancelled" and 4 for
// auth failures; renumbering is deferred to the exit-codes slice of the
// gh-compat work (issue #5).
func TestUsageErrorExitCodes(t *testing.T) {
	repo := adminUser + "/test-repo"

	stdout, stderr, err := runFJ("pr", "view", "abc", "-R", repo)
	if err == nil {
		t.Fatalf("expected failure for a non-numeric argument, got:\n%s", stdout)
	}
	if code := exitCode(t, err); code != 2 {
		t.Errorf("invalid-number exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "invalid pull request number") {
		t.Errorf("expected an invalid-number error, got: %s", stderr)
	}

	stdout, stderr, err = runFJ("pr", "checkout", "-R", repo)
	if err == nil {
		t.Fatalf("expected failure for checkout without a number, got:\n%s", stdout)
	}
	if code := exitCode(t, err); code != 2 {
		t.Errorf("missing-number exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage output for checkout without a number, got: %s", stderr)
	}
}

// TestPRCheckoutCurrentBranch covers the short circuit: checking out the
// pull request whose head is already the checked-out branch must not run
// git fetch (which would exit 128 refusing to fetch into the current
// branch).
func TestPRCheckoutCurrentBranch(t *testing.T) {
	dir := gitRepoOnBranch(t, "feature-1") // PR #4

	stdout, stderr, err := runFJIn(dir, "pr", "checkout", "4")
	if err != nil {
		t.Fatalf("fj pr checkout 4 failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "already checked out") {
		t.Errorf("expected an already-checked-out notice, got: %s", stderr)
	}
}

// TestPRMergeCloseImplicitRequireYes covers the confirmation gate: with no
// number, merge and close report the pull request they resolved from the
// current branch and refuse to act without --yes.
func TestPRMergeCloseImplicitRequireYes(t *testing.T) {
	repo := adminUser + "/test-repo"

	cases := []struct {
		verb   string
		branch string
		pr     string
	}{
		{"merge", "feature-1", "#4"},
		{"close", "feature-2", "#5"},
	}
	for _, tc := range cases {
		dir := gitRepoOnBranch(t, tc.branch)

		stdout, stderr, err := runFJIn(dir, "pr", tc.verb)
		if err == nil {
			t.Fatalf("fj pr %s without --yes succeeded:\n%s", tc.verb, stdout)
		}
		if code := exitCode(t, err); code != 2 {
			t.Errorf("pr %s without --yes exit code = %d, want 2", tc.verb, code)
		}
		for _, want := range []string{"Resolved pull request " + tc.pr, "--yes"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("pr %s stderr missing %q: %s", tc.verb, want, stderr)
			}
		}
	}

	// Neither pull request was touched.
	for _, number := range []string{"4", "5"} {
		out := mustRunFJ(t, "pr", "view", number, "-R", repo)
		if !strings.Contains(out, "State: open") {
			t.Errorf("PR %s is no longer open:\n%s", number, out)
		}
	}
}

func TestPRCloseImplicitWithYes(t *testing.T) {
	branch := "close-implicit-yes"
	if _, _, err := testClient.CreateBranch(adminUser, "test-repo", forgejo.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: "main",
	}); err != nil {
		t.Fatalf("creating branch: %v", err)
	}
	if _, _, err := testClient.CreateFile(adminUser, "test-repo", branch+".txt", forgejo.CreateFileOptions{
		FileOptions: forgejo.FileOptions{Message: "Add " + branch, BranchName: branch},
		Content:     base64.StdEncoding.EncodeToString([]byte(branch + "\n")),
	}); err != nil {
		t.Fatalf("creating file: %v", err)
	}
	pr, _, err := testClient.CreatePullRequest(adminUser, "test-repo", forgejo.CreatePullRequestOption{
		Head:  branch,
		Base:  "main",
		Title: "Close via implicit --yes",
	})
	if err != nil {
		t.Fatalf("creating PR: %v", err)
	}

	dir := gitRepoOnBranch(t, branch)
	stdout, stderr, err := runFJIn(dir, "pr", "close", "--yes")
	if err != nil {
		t.Fatalf("fj pr close --yes failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	want := fmt.Sprintf("Closed pull request #%d", pr.Index)
	if !strings.Contains(stderr, want) {
		t.Errorf("expected %q, got: %s", want, stderr)
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
// failing on a missing body. Inherited flags like -R do not make an
// invocation non-bare, so those spellings print help too — consistently
// across issue comment, pr comment, and pr review.
func TestCommentGroupHelp(t *testing.T) {
	repo := adminUser + "/test-repo"

	stdout := mustRunFJ(t, "issue", "comment")
	for _, want := range []string{"Available Commands:", "create", "--body"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in group help:\n%s", want, stdout)
		}
	}

	for _, args := range [][]string{
		{"issue", "comment", "-R", repo},
		{"pr", "comment", "-R", repo},
		{"pr", "review", "-R", repo},
	} {
		stdout := mustRunFJ(t, args...)
		if !strings.Contains(stdout, "Available Commands:") {
			t.Errorf("fj %s: expected group help, got:\n%s", strings.Join(args, " "), stdout)
		}
	}
}

// TestGroupUnknownSubcommand keeps mistyped subcommands from falling
// through to the group's leaf verb and failing with a misleading
// missing-flag error.
func TestGroupUnknownSubcommand(t *testing.T) {
	repo := adminUser + "/test-repo"

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"issue", "comment", "lst", "1", "-R", repo}, `unknown command "lst" for "fj issue comment"`},
		{[]string{"pr", "comment", "lst", "4", "-R", repo}, `unknown command "lst" for "fj pr comment"`},
		{[]string{"pr", "review", "lst", "-R", repo}, `unknown command "lst" for "fj pr review"`},
	} {
		stdout, stderr, err := runFJ(tc.args...)
		if err == nil {
			t.Fatalf("fj %s succeeded, want unknown-command error:\n%s", strings.Join(tc.args, " "), stdout)
		}
		if !strings.Contains(stderr, tc.want) {
			t.Errorf("fj %s stderr = %q, want it to contain %q", strings.Join(tc.args, " "), stderr, tc.want)
		}
		// 1, not the 2 that TestUsageErrorExitCodes pins for arity errors:
		// this matches what cobra already does for an unknown command at the
		// root (`fj bogus`). Unifying the two is part of the exit-codes slice
		// of issue #5.
		if code := exitCode(t, err); code != 1 {
			t.Errorf("fj %s exit code = %d, want 1", strings.Join(tc.args, " "), code)
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
