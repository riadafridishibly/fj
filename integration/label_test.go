//go:build integration

package integration

import (
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
