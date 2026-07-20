package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func requireMergeTree(t *testing.T) {
	t.Helper()
	if ok, ver := SupportsMergeTree(); !ok {
		t.Skipf("git %s lacks merge-tree --write-tree (needs 2.38+)", ver)
	}
}

func gitT(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// initConflictRepo builds a repo whose "feature" branch conflicts with "main"
// on shared.txt (via a name with a space to exercise -z framing) and adds a
// non-conflicting file. It returns the repo dir; the test cwd is set to it.
func initConflictRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)

	gitT(t, "init", "-q", "-b", "main")
	gitT(t, "config", "user.email", "t@t")
	gitT(t, "config", "user.name", "t")

	writeFile(t, dir, "shared file.txt", "line1\nline2\nline3\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "base")

	gitT(t, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "shared file.txt", "line1\nFEATURE\nline3\n")
	writeFile(t, dir, "only-feature.txt", "new\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "feat")

	gitT(t, "checkout", "-q", "main")
	writeFile(t, dir, "shared file.txt", "line1\nMAIN\nline3\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "main-change")

	return dir
}

func TestMergeTreeConflictsReportsConflict(t *testing.T) {
	requireMergeTree(t)
	initConflictRepo(t)

	files, clean, err := MergeTreeConflicts("main", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clean {
		t.Fatal("expected a conflict, got clean")
	}
	want := []string{"shared file.txt"}
	if !slices.Equal(files, want) {
		t.Fatalf("conflicting files = %q, want %q", files, want)
	}
}

func TestMergeTreeConflictsCleanMerge(t *testing.T) {
	requireMergeTree(t)
	dir := initConflictRepo(t)

	gitT(t, "checkout", "-q", "-b", "clean-side", "main")
	writeFile(t, dir, "clean-side.txt", "side\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "side")

	gitT(t, "checkout", "-q", "main")
	writeFile(t, dir, "main-only.txt", "main\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "main-only")

	files, clean, err := MergeTreeConflicts("main", "clean-side")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean merge, got conflicts %q", files)
	}
	if files != nil {
		t.Fatalf("expected no files, got %q", files)
	}
}

func TestMergeTreeConflictsMultipleFiles(t *testing.T) {
	requireMergeTree(t)
	dir := initConflictRepo(t)

	gitT(t, "checkout", "-q", "feature")
	writeFile(t, dir, "second.txt", "FEATURE\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "feat-second")

	gitT(t, "checkout", "-q", "main")
	writeFile(t, dir, "second.txt", "MAIN\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "main-second")

	files, clean, err := MergeTreeConflicts("main", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clean {
		t.Fatal("expected conflicts, got clean")
	}
	slices.Sort(files)
	want := []string{"second.txt", "shared file.txt"}
	if !slices.Equal(files, want) {
		t.Fatalf("conflicting files = %q, want %q", files, want)
	}
}

func TestMergeTreeConflictsUnknownRev(t *testing.T) {
	requireMergeTree(t)
	initConflictRepo(t)

	if _, _, err := MergeTreeConflicts("main", "does-not-exist"); err == nil {
		t.Fatal("expected an error for a missing rev, got nil")
	}
}

func TestMergeTreeConflictsUnrelatedHistories(t *testing.T) {
	requireMergeTree(t)
	dir := initConflictRepo(t)

	gitT(t, "checkout", "-q", "--orphan", "orphan")
	gitT(t, "rm", "-r", "-f", "-q", ".")
	writeFile(t, dir, "orphan.txt", "o\n")
	gitT(t, "add", "-A")
	gitT(t, "commit", "-qm", "orphan")

	if _, _, err := MergeTreeConflicts("main", "orphan"); err == nil {
		t.Fatal("expected an error for unrelated histories, got nil")
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"2.38.0", true},
		{"2.50.1", true},
		{"3.0.0", true},
		{"2.37.9", false},
		{"2.34.1", false},
		{"1.9.5", false},
		{"weird", true},
		{"2", true},
	}
	for _, c := range cases {
		if got := versionAtLeast(c.v, 2, 38); got != c.want {
			t.Errorf("versionAtLeast(%q, 2, 38) = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestRevExists(t *testing.T) {
	initConflictRepo(t)

	if !RevExists("main") {
		t.Error("main should exist")
	}
	if !RevExists("HEAD") {
		t.Error("HEAD should exist")
	}
	if RevExists("nope") {
		t.Error("nope should not exist")
	}
	if RevExists("") {
		t.Error("empty rev should not exist")
	}
}
