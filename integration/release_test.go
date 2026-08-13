//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReleaseCreate creates a release via the CLI and verifies it via the SDK.
func TestReleaseCreate(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.1.0-create"

	stdout, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--title", "Release "+tag,
		"--notes", "Created by integration test")
	if err != nil {
		t.Fatalf("release create failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "/releases/tag/"+tag) {
		t.Errorf("expected release URL in output:\n%s", stdout)
	}

	// Verify via SDK
	rel, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag)
	if err != nil {
		t.Fatalf("SDK GetReleaseByTag failed: %v", err)
	}
	if rel.Title != "Release "+tag {
		t.Errorf("expected title %q, got %q", "Release "+tag, rel.Title)
	}
	if rel.Note != "Created by integration test" {
		t.Errorf("expected note to match, got %q", rel.Note)
	}
	if rel.IsDraft {
		t.Error("expected release to be published, not draft")
	}
}

// TestReleaseCreateWithAssets creates a release with positional asset files.
func TestReleaseCreateWithAssets(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.1.0-assets"

	tmpDir := t.TempDir()
	assetPath := filepath.Join(tmpDir, "binary.tar.gz")
	if err := os.WriteFile(assetPath, []byte("fake binary content"), 0o644); err != nil {
		t.Fatalf("failed to write asset file: %v", err)
	}

	stdout, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--notes", "With assets",
		assetPath)
	if err != nil {
		t.Fatalf("release create failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "Uploaded binary.tar.gz") {
		t.Errorf("expected upload confirmation in stderr:\n%s", stderr)
	}

	rel, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag)
	if err != nil {
		t.Fatalf("SDK GetReleaseByTag failed: %v", err)
	}
	if len(rel.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(rel.Attachments))
	}
	if rel.Attachments[0].Name != "binary.tar.gz" {
		t.Errorf("expected asset name 'binary.tar.gz', got %q", rel.Attachments[0].Name)
	}
}

// TestReleaseCreateDraft verifies --draft flag.
func TestReleaseCreateDraft(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.1.0-draft"

	_, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--draft",
		"--notes", "Draft release")
	if err != nil {
		t.Fatalf("release create failed: %v\nstderr: %s", err, stderr)
	}

	rel, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag)
	if err != nil {
		t.Fatalf("SDK GetReleaseByTag failed: %v", err)
	}
	if !rel.IsDraft {
		t.Error("expected release to be a draft")
	}
}

// TestReleaseCreateNotesFile verifies --notes-file flag.
func TestReleaseCreateNotesFile(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.1.0-notesfile"

	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "CHANGELOG.md")
	notes := "# Changelog\n\n- First change\n- Second change\n"
	if err := os.WriteFile(notesPath, []byte(notes), 0o644); err != nil {
		t.Fatalf("failed to write notes file: %v", err)
	}

	_, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--notes-file", notesPath)
	if err != nil {
		t.Fatalf("release create failed: %v\nstderr: %s", err, stderr)
	}

	rel, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag)
	if err != nil {
		t.Fatalf("SDK GetReleaseByTag failed: %v", err)
	}
	if rel.Note != notes {
		t.Errorf("expected notes from file, got %q", rel.Note)
	}
}

// TestReleaseList lists releases and checks they appear.
func TestReleaseList(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.2.0-list"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--notes", "for list test"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "release", "list", "-R", repo)
	if !strings.Contains(stdout, tag) {
		t.Errorf("expected tag %q in list output:\n%s", tag, stdout)
	}
}

// TestReleaseListJSON verifies --json output is valid.
func TestReleaseListJSON(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.2.0-listjson"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--notes", "for json list test"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "release", "list", "-R", repo, "--json")

	var releases []map[string]any
	if err := json.Unmarshal([]byte(stdout), &releases); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	if len(releases) == 0 {
		t.Fatal("expected at least one release in JSON list")
	}

	var found bool
	for _, r := range releases {
		if r["tag_name"] == tag {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected release with tag %q in JSON output", tag)
	}
}

// TestReleaseListExcludeDrafts verifies --exclude-drafts filter.
func TestReleaseListExcludeDrafts(t *testing.T) {
	repo := adminUser + "/test-repo"
	draftTag := "v0.3.0-drafthidden"
	publishedTag := "v0.3.0-published"

	if _, stderr, err := runFJ("release", "create", draftTag,
		"-R", repo, "--draft", "--notes", "draft"); err != nil {
		t.Fatalf("draft create failed: %v\n%s", err, stderr)
	}
	if _, stderr, err := runFJ("release", "create", publishedTag,
		"-R", repo, "--notes", "published"); err != nil {
		t.Fatalf("published create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "release", "list", "-R", repo, "--exclude-drafts", "--json")

	var releases []map[string]any
	if err := json.Unmarshal([]byte(stdout), &releases); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, r := range releases {
		if r["tag_name"] == draftTag {
			t.Errorf("draft release %q should not appear with --exclude-drafts", draftTag)
		}
	}
}

// TestReleaseView shows a release by tag.
func TestReleaseView(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.4.0-view"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--title", "View Test Release",
		"--notes", "View body content"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "release", "view", tag, "-R", repo)

	for _, want := range []string{"View Test Release", tag, "View body content"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in view output:\n%s", want, stdout)
		}
	}
}

// TestReleaseViewLatest (no tag arg) returns the latest release.
func TestReleaseViewLatest(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v9.9.9-latest"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--title", "Latest Test",
		"--notes", "latest"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "release", "view", "-R", repo)
	if !strings.Contains(stdout, tag) {
		t.Errorf("expected latest tag %q in view output:\n%s", tag, stdout)
	}
}

// TestReleaseViewJSON verifies --json output for view.
func TestReleaseViewJSON(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.4.0-viewjson"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--title", "JSON View Test",
		"--notes", "json"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "release", "view", tag, "-R", repo, "--json")

	var rel map[string]any
	if err := json.Unmarshal([]byte(stdout), &rel); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}
	if rel["tag_name"] != tag {
		t.Errorf("expected tag_name %q, got %v", tag, rel["tag_name"])
	}
	if rel["name"] != "JSON View Test" {
		t.Errorf("expected name 'JSON View Test', got %v", rel["name"])
	}
}

// TestReleaseEdit changes the title and notes of an existing release.
func TestReleaseEdit(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.5.0-edit"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--title", "Original Title",
		"--notes", "original notes"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	_, stderr, err := runFJ("release", "edit", tag,
		"-R", repo,
		"--title", "Updated Title",
		"--notes", "updated notes")
	if err != nil {
		t.Fatalf("release edit failed: %v\nstderr: %s", err, stderr)
	}

	rel, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag)
	if err != nil {
		t.Fatalf("SDK GetReleaseByTag failed: %v", err)
	}
	if rel.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", rel.Title)
	}
	if rel.Note != "updated notes" {
		t.Errorf("expected notes 'updated notes', got %q", rel.Note)
	}
}

// TestReleaseEditPromoteDraft verifies --draft=false promotes a draft.
func TestReleaseEditPromoteDraft(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.5.0-promote"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo, "--draft", "--notes", "will be promoted"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	_, stderr, err := runFJ("release", "edit", tag, "-R", repo, "--draft=false")
	if err != nil {
		t.Fatalf("release edit failed: %v\nstderr: %s", err, stderr)
	}

	rel, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag)
	if err != nil {
		t.Fatalf("SDK GetReleaseByTag failed: %v", err)
	}
	if rel.IsDraft {
		t.Error("expected release to be promoted (not draft)")
	}
}

// TestReleaseUpload uploads an additional asset to an existing release.
func TestReleaseUpload(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.6.0-upload"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo, "--notes", "for upload test"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	tmpDir := t.TempDir()
	assetPath := filepath.Join(tmpDir, "artifact.zip")
	if err := os.WriteFile(assetPath, []byte("zipped content"), 0o644); err != nil {
		t.Fatalf("failed to write asset file: %v", err)
	}

	_, stderr, err := runFJ("release", "upload", tag, assetPath, "-R", repo)
	if err != nil {
		t.Fatalf("release upload failed: %v\nstderr: %s", err, stderr)
	}

	rel, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag)
	if err != nil {
		t.Fatalf("SDK GetReleaseByTag failed: %v", err)
	}
	var found bool
	for _, a := range rel.Attachments {
		if a.Name == "artifact.zip" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected artifact.zip in release attachments, got %d attachments", len(rel.Attachments))
	}
}

// TestReleaseUploadClobberRequired verifies upload fails without --clobber
// when asset already exists, and succeeds with --clobber.
func TestReleaseUploadClobberRequired(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.6.0-clobber"

	tmpDir := t.TempDir()
	assetPath := filepath.Join(tmpDir, "dup.txt")
	if err := os.WriteFile(assetPath, []byte("first"), 0o644); err != nil {
		t.Fatalf("failed to write asset file: %v", err)
	}

	// Create release with initial asset
	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo, "--notes", "clobber test", assetPath); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	// Second upload without --clobber should fail
	_, _, err := runFJ("release", "upload", tag, assetPath, "-R", repo)
	if err == nil {
		t.Error("expected upload to fail without --clobber when asset exists")
	}

	// Rewrite the file and upload with --clobber
	if err := os.WriteFile(assetPath, []byte("second"), 0o644); err != nil {
		t.Fatalf("failed to rewrite asset: %v", err)
	}
	_, stderr, err := runFJ("release", "upload", tag, assetPath, "-R", repo, "--clobber")
	if err != nil {
		t.Fatalf("clobber upload failed: %v\nstderr: %s", err, stderr)
	}
}

// TestReleaseDownload downloads assets from a release.
func TestReleaseDownload(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.7.0-download"

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "payload.bin")
	payload := []byte("payload content xyz")
	if err := os.WriteFile(srcPath, payload, 0o644); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo, "--notes", "download test", srcPath); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	destDir := filepath.Join(tmpDir, "downloads")
	_, stderr, err := runFJ("release", "download", tag, "-R", repo, "--dir", destDir)
	if err != nil {
		t.Fatalf("release download failed: %v\nstderr: %s", err, stderr)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "payload.bin"))
	if err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("downloaded content mismatch: got %q, want %q", got, payload)
	}
}

// TestReleaseDelete deletes a release and verifies it's gone. It also
// exercises the shared delete-flag contract: --yes is required to delete and
// --dry-run previews without deleting.
func TestReleaseDelete(t *testing.T) {
	repo := adminUser + "/test-repo"
	tag := "v0.8.0-delete"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo, "--notes", "to be deleted"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	// Without --yes or --dry-run, the command must refuse to delete.
	if _, _, err := runFJ("release", "delete", tag, "-R", repo); err == nil {
		t.Fatalf("release delete without --yes should fail")
	}

	// --dry-run must resolve and print the release but leave it intact.
	stdout, stderr, err := runFJ("release", "delete", tag, "-R", repo, "--dry-run")
	if err != nil {
		t.Fatalf("release delete --dry-run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, tag) {
		t.Errorf("dry-run output should mention the tag\nstderr: %s", stderr)
	}
	if !strings.Contains(stderr, "dry-run") {
		t.Errorf("dry-run output should state nothing was changed\nstderr: %s", stderr)
	}
	if _, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag); err != nil {
		t.Errorf("dry-run must not delete the release: %v", err)
	}

	// --yes performs the actual deletion.
	_, stderr, err = runFJ("release", "delete", tag, "-R", repo, "--yes")
	if err != nil {
		t.Fatalf("release delete failed: %v\nstderr: %s", err, stderr)
	}

	if _, _, err := testClient.GetReleaseByTag(adminUser, "test-repo", tag); err == nil {
		t.Errorf("expected release %s to be deleted", tag)
	}
}

// TestStatusIncludesLatestRelease verifies fj repo status surfaces the latest release.
func TestStatusIncludesLatestRelease(t *testing.T) {
	repo := adminUser + "/another-repo"
	tag := "v1.0.0-status"

	if _, stderr, err := runFJ("release", "create", tag,
		"-R", repo,
		"--title", "Status Release",
		"--notes", "for status test"); err != nil {
		t.Fatalf("setup create failed: %v\n%s", err, stderr)
	}

	stdout := mustRunFJ(t, "repo", "status", "-R", repo, "--json")

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout)
	}

	latest, ok := result["latest_release"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'latest_release' key in status JSON, got: %v", result)
	}
	if latest["tag"] != tag {
		t.Errorf("expected tag %q in latest_release, got %v", tag, latest["tag"])
	}
	if latest["title"] != "Status Release" {
		t.Errorf("expected title 'Status Release', got %v", latest["title"])
	}
}
