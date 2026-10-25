//go:build integration

package integration

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// labelExists reports whether a label with the given name exists in test-repo,
// matching case-insensitively (the same rule fj label delete uses).
func labelExists(t *testing.T, name string) bool {
	t.Helper()
	labels, _, err := testClient.ListRepoLabels(adminUser, "test-repo", forgejo.ListLabelsOptions{})
	if err != nil {
		t.Fatalf("listing labels: %v", err)
	}
	for _, l := range labels {
		if strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}

func TestLabelDelete(t *testing.T) {
	repo := adminUser + "/test-repo"
	name := "label-delete-contract"

	if _, stderr, err := runFJ("label", "create", "--name", name, "-R", repo, "--color", "#5319e7"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}
	if !labelExists(t, name) {
		t.Fatal("setup: label was not created")
	}

	// Without --yes or --dry-run, the command must refuse to delete.
	if _, _, err := runFJ("label", "delete", name, "-R", repo); err == nil {
		t.Fatalf("label delete without --yes should fail")
	}

	// --dry-run must resolve and print the label but leave it intact.
	stdout, stderr, err := runFJ("label", "delete", name, "-R", repo, "--dry-run")
	if err != nil {
		t.Fatalf("label delete --dry-run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, name) {
		t.Errorf("dry-run output should mention the label name\nstderr: %s", stderr)
	}
	if !strings.Contains(stderr, "dry-run") {
		t.Errorf("dry-run output should state nothing was changed\nstderr: %s", stderr)
	}
	if !labelExists(t, name) {
		t.Errorf("dry-run must not delete the label")
	}

	// --yes performs the actual deletion.
	_, stderr, err = runFJ("label", "delete", name, "-R", repo, "--yes")
	if err != nil {
		t.Fatalf("label delete failed: %v\nstderr: %s", err, stderr)
	}
	if labelExists(t, name) {
		t.Errorf("expected label %q to be deleted", name)
	}
}

// TestLabelLookupPastFirstPage gives a repository more labels than the
// server's default page of 30 and uses the one that sorts last. fj must find
// it on a later page instead of creating a duplicate.
func TestLabelLookupPastFirstPage(t *testing.T) {
	name := "label-lookup"
	if _, _, err := testClient.CreateRepo(forgejo.CreateRepoOption{Name: name, AutoInit: true}); err != nil {
		t.Fatalf("creating repo: %v", err)
	}
	repo := adminUser + "/" + name
	for i := range 31 {
		if _, _, err := testClient.CreateLabel(adminUser, name, forgejo.CreateLabelOption{
			Name:  fmt.Sprintf("filler-%02d", i),
			Color: "#cccccc",
		}); err != nil {
			t.Fatalf("creating label: %v", err)
		}
	}
	late, _, err := testClient.CreateLabel(adminUser, name, forgejo.CreateLabelOption{Name: "zz-late", Color: "#ee0701"})
	if err != nil {
		t.Fatalf("creating label: %v", err)
	}

	labelCount := func() int {
		t.Helper()
		labels, _, err := testClient.ListRepoLabels(adminUser, name, forgejo.ListLabelsOptions{
			ListOptions: forgejo.ListOptions{Page: 1, PageSize: 50},
		})
		if err != nil {
			t.Fatalf("listing labels: %v", err)
		}
		return len(labels)
	}
	issueLabels := func(num string) []int64 { return issueLabelIDs(t, adminUser, name, num) }

	num := strings.TrimSpace(mustRunFJ(t, "issue", "create", "-R", repo, "--title", "late label",
		"--label", "zz-late", "--json", "number", "--jq", ".number"))
	if got := issueLabels(num); !slices.Equal(got, []int64{late.ID}) {
		t.Errorf("issue create --label zz-late: issue labels = %v, want [%d]", got, late.ID)
	}
	if n := labelCount(); n != 32 {
		t.Fatalf("issue create --label zz-late: repo has %d labels, want 32", n)
	}

	mustRunFJ(t, "issue", "edit", num, "-R", repo, "--remove-label", "zz-late")
	if got := issueLabels(num); len(got) != 0 {
		t.Errorf("issue edit --remove-label zz-late: issue labels = %v, want none", got)
	}
	mustRunFJ(t, "issue", "edit", num, "-R", repo, "--add-label", "zz-late")
	if got := issueLabels(num); !slices.Equal(got, []int64{late.ID}) {
		t.Errorf("issue edit --add-label zz-late: issue labels = %v, want [%d]", got, late.ID)
	}
	if n := labelCount(); n != 32 {
		t.Fatalf("issue edit --add-label zz-late: repo has %d labels, want 32", n)
	}

	mustRunFJ(t, "label", "edit", "zz-late", "-R", repo, "--description", "sorts last")
	mustRunFJ(t, "label", "delete", "zz-late", "-R", repo, "--yes")
	if n := labelCount(); n != 31 {
		t.Errorf("label delete zz-late: repo has %d labels, want 31", n)
	}
}

// TestLabelUnknownName pins gh's errors for a label the repository lacks: fj
// creates no label, no issue, and no partial edit.
func TestLabelUnknownName(t *testing.T) {
	repo := adminUser + "/test-repo"
	labelsBefore := len(listTestRepoLabels(t))

	_, stderr, err := runFJ("issue", "create", "-R", repo, "--title", "unknown label", "--label", "no-such-label")
	if err == nil || !strings.Contains(stderr, "could not add label: 'no-such-label' not found") {
		t.Errorf("issue create: err = %v, stderr = %q, want gh's could not add label", err, stderr)
	}
	for page := 1; ; page++ {
		issues, _, err := testClient.ListRepoIssues(adminUser, "test-repo", forgejo.ListIssueOption{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: 50},
			State:       forgejo.StateAll,
		})
		if err != nil {
			t.Fatalf("listing issues: %v", err)
		}
		if len(issues) == 0 {
			break
		}
		for _, i := range issues {
			if i.Title == "unknown label" {
				t.Errorf("issue create with an unknown label created issue #%d", i.Index)
			}
		}
	}

	for _, flag := range []string{"--add-label", "--remove-label"} {
		_, stderr, err := runFJ("issue", "edit", "1", "-R", repo, "--title", "edited title", flag, "no-such-label")
		if err == nil || !strings.Contains(stderr, "'no-such-label' not found") {
			t.Errorf("issue edit %s: err = %v, stderr = %q, want gh's not found", flag, err, stderr)
		}
	}
	issue, _, err := testClient.GetIssue(adminUser, "test-repo", 1)
	if err != nil {
		t.Fatalf("getting issue: %v", err)
	}
	if issue.Title == "edited title" {
		t.Error("issue edit applied the title before failing on the unknown label")
	}

	for _, flag := range []string{"--add-label", "--remove-label"} {
		_, stderr, err := runFJ("pr", "edit", "4", "-R", repo, "--title", "edited title", flag, "no-such-label")
		if err == nil || !strings.Contains(stderr, "'no-such-label' not found") {
			t.Errorf("pr edit %s: err = %v, stderr = %q, want gh's not found", flag, err, stderr)
		}
	}
	pr, _, err := testClient.GetPullRequest(adminUser, "test-repo", 4)
	if err != nil {
		t.Fatalf("getting pull request: %v", err)
	}
	if pr.Title == "edited title" {
		t.Error("pr edit applied the title before failing on the unknown label")
	}

	if n := len(listTestRepoLabels(t)); n != labelsBefore {
		t.Errorf("repo has %d labels after the failed commands, want %d", n, labelsBefore)
	}
}

func listTestRepoLabels(t *testing.T) []*forgejo.Label {
	t.Helper()
	labels, _, err := testClient.ListRepoLabels(adminUser, "test-repo", forgejo.ListLabelsOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: 50},
	})
	if err != nil {
		t.Fatalf("listing labels: %v", err)
	}
	return labels
}

// TestLabelOrgLabels uses labels in an organization's repository. An
// organization label resolves when the repository has no label of that name,
// and a repository label wins over an organization label with the same name.
func TestLabelOrgLabels(t *testing.T) {
	org, name := "label-org", "app"
	if _, _, err := testClient.CreateOrg(forgejo.CreateOrgOption{Name: org}); err != nil {
		t.Fatalf("creating org: %v", err)
	}
	if _, _, err := testClient.CreateOrgRepo(org, forgejo.CreateRepoOption{Name: name, AutoInit: true}); err != nil {
		t.Fatalf("creating repo: %v", err)
	}
	repo := org + "/" + name
	repoShared, _, err := testClient.CreateLabel(org, name, forgejo.CreateLabelOption{Name: "shared", Color: "#0075ca"})
	if err != nil {
		t.Fatalf("creating label: %v", err)
	}
	orgLabel := func(label string) int64 {
		t.Helper()
		out := mustRunFJ(t, "api", "orgs/"+org+"/labels", "-f", "name="+label, "-f", "color=#5319e7", "--jq", ".id")
		id, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		if err != nil {
			t.Fatalf("creating org label %s: id %q: %v", label, out, err)
		}
		return id
	}
	orgSharedID := orgLabel("shared")
	orgOnlyID := orgLabel("org-only")

	num := strings.TrimSpace(mustRunFJ(t, "issue", "create", "-R", repo, "--title", "org labels",
		"--label", "org-only", "--label", "shared", "--json", "number", "--jq", ".number"))
	want := []int64{repoShared.ID, orgOnlyID}
	slices.Sort(want)
	if got := issueLabelIDs(t, org, name, num); !slices.Equal(got, want) {
		t.Errorf("issue create: labels = %v, want %v (repo shared, org org-only; org shared is %d)", got, want, orgSharedID)
	}

	mustRunFJ(t, "issue", "edit", num, "-R", repo, "--remove-label", "org-only")
	if got := issueLabelIDs(t, org, name, num); !slices.Equal(got, []int64{repoShared.ID}) {
		t.Errorf("issue edit --remove-label org-only: labels = %v, want [%d]", got, repoShared.ID)
	}
	mustRunFJ(t, "issue", "edit", num, "-R", repo, "--add-label", "org-only")
	if got := issueLabelIDs(t, org, name, num); !slices.Equal(got, want) {
		t.Errorf("issue edit --add-label org-only: labels = %v, want %v", got, want)
	}
}

// issueLabelIDs returns the sorted label IDs of issue num in owner/repo.
func issueLabelIDs(t *testing.T, owner, repo, num string) []int64 {
	t.Helper()
	index, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		t.Fatalf("issue number %q: %v", num, err)
	}
	issue, _, err := testClient.GetIssue(owner, repo, index)
	if err != nil {
		t.Fatalf("getting issue %s: %v", num, err)
	}
	var ids []int64
	for _, l := range issue.Labels {
		ids = append(ids, l.ID)
	}
	slices.Sort(ids)
	return ids
}
