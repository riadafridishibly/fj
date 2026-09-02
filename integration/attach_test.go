//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// fetchAttachment downloads an attachment URL from the test server. The
// container advertises its own ROOT_URL (localhost:3000) in every URL it
// hands out, while the test reaches it on a mapped port, so the host is
// swapped for the one the test client uses before the request goes out.
func fetchAttachment(t *testing.T, rawURL string) []byte {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing %q: %v", rawURL, err)
	}
	u.Scheme, u.Host = "http", forgejoHost

	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		t.Fatalf("GET %s: status %s: %s", u, resp.Status, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", u, err)
	}
	return body
}

// newAttachTarget creates an issue through the SDK and returns its index, so
// an attachment test starts from an issue nothing else has touched.
func newAttachTarget(t *testing.T, title string) int64 {
	t.Helper()

	issue, _, err := testClient.CreateIssue(adminUser, "test-repo", forgejo.CreateIssueOption{
		Title: title,
		Body:  "created by the integration suite",
	})
	if err != nil {
		t.Fatalf("creating issue %q: %v", title, err)
	}
	return issue.Index
}

// TestIssueAttach uploads two files to an issue and checks that the URLs it
// prints are the ones a user would paste into a body: fetching the first
// returns the bytes that went up.
func TestIssueAttach(t *testing.T) {
	repo := adminUser + "/test-repo"
	index := newAttachTarget(t, "Attachment target")

	dir := t.TempDir()
	first := filepath.Join(dir, "screenshot.png")
	second := filepath.Join(dir, "crash.log")
	payload := []byte("not really a png, but the bytes must survive")
	if err := os.WriteFile(first, payload, 0o644); err != nil {
		t.Fatalf("writing %s: %v", first, err)
	}
	if err := os.WriteFile(second, []byte("panic: nope\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", second, err)
	}

	stdout, stderr, err := runFJ("issue", "attach", strconv.FormatInt(index, 10),
		first, second, "-R", repo)
	if err != nil {
		t.Fatalf("issue attach failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	urls := strings.Fields(strings.TrimSpace(stdout))
	if len(urls) != 2 {
		t.Fatalf("got %d URLs on stdout, want 2:\n%s", len(urls), stdout)
	}
	for i, u := range urls {
		if !strings.HasPrefix(u, "http") {
			t.Errorf("URL %d = %q, want a browser_download_url", i, u)
		}
	}
	// Progress lines belong on stderr so the URLs pipe cleanly.
	if !strings.Contains(stderr, "screenshot.png") {
		t.Errorf("expected an upload line for screenshot.png on stderr:\n%s", stderr)
	}

	if got := fetchAttachment(t, urls[0]); string(got) != string(payload) {
		t.Errorf("downloaded content = %q, want %q", got, payload)
	}
}

// TestIssueAttachJSON checks the --json shape: the full attachment objects,
// each carrying the download URL.
func TestIssueAttachJSON(t *testing.T) {
	repo := adminUser + "/test-repo"
	index := newAttachTarget(t, "Attachment target for JSON")

	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("some notes\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	stdout := mustRunFJ(t, "issue", "attach", strconv.FormatInt(index, 10),
		path, "-R", repo, "--json")

	var attachments []forgejo.Attachment
	if err := json.Unmarshal([]byte(stdout), &attachments); err != nil {
		t.Fatalf("parsing --json output: %v\n%s", err, stdout)
	}
	if len(attachments) != 1 {
		t.Fatalf("got %d attachments, want 1:\n%s", len(attachments), stdout)
	}
	if attachments[0].Name != "notes.txt" {
		t.Errorf("name = %q, want %q", attachments[0].Name, "notes.txt")
	}
	if attachments[0].DownloadURL == "" {
		t.Error("browser_download_url is empty")
	}
	if got := fetchAttachment(t, attachments[0].DownloadURL); string(got) != "some notes\n" {
		t.Errorf("downloaded content = %q, want %q", got, "some notes\n")
	}
}

// TestIssueAttachValidatesPathsUpFront covers the promise that a bad path
// fails before anything uploads: the good file named ahead of the missing
// one must not have been sent.
func TestIssueAttachValidatesPathsUpFront(t *testing.T) {
	repo := adminUser + "/test-repo"
	index := newAttachTarget(t, "Attachment target for validation")

	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(good, []byte("fine\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", good, err)
	}
	missing := filepath.Join(dir, "typo.txt")

	stdout, stderr, err := runFJ("issue", "attach", strconv.FormatInt(index, 10),
		good, missing, "-R", repo)
	if err == nil {
		t.Fatalf("expected a failure for a missing path\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("nothing should have uploaded, but stdout has URLs:\n%s", stdout)
	}
	if !strings.Contains(stderr, "typo.txt") {
		t.Errorf("expected the failing path named on stderr:\n%s", stderr)
	}
}

// forgejoMaxAttachmentSize is Forgejo's default [attachment] MAX_SIZE, in
// MiB. The test container does not override it, so a file comfortably above
// it is a deterministic way to make one upload fail on the server after an
// earlier one has already succeeded.
const forgejoMaxAttachmentSize = 4 << 20

// TestIssueAttachPartialFailureJSON is the subtlest case in the command: the
// first file lands, the second is rejected by the server, and --json is set.
// The attachments that did land must still reach stdout as JSON, the notice
// that they stay attached must go to stderr, and the command must exit
// non-zero.
func TestIssueAttachPartialFailureJSON(t *testing.T) {
	repo := adminUser + "/test-repo"
	index := newAttachTarget(t, "Attachment target for partial failure")

	dir := t.TempDir()
	small := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(small, []byte("this one lands\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", small, err)
	}
	// Oversized, but a valid path holding a readable regular file: the
	// up-front check passes it, and only the server refuses it.
	oversized := filepath.Join(dir, "oversized.bin")
	if err := os.WriteFile(oversized, make([]byte, forgejoMaxAttachmentSize+(1<<20)), 0o644); err != nil {
		t.Fatalf("writing %s: %v", oversized, err)
	}

	stdout, stderr, err := runFJ("issue", "attach", strconv.FormatInt(index, 10),
		small, oversized, "-R", repo, "--json")
	if err == nil {
		t.Fatalf("expected a non-zero exit when the second upload is refused\nstdout: %s\nstderr: %s", stdout, stderr)
	}

	// A partial run must not cost the caller the URLs it already earned.
	var attachments []forgejo.Attachment
	if jsonErr := json.Unmarshal([]byte(stdout), &attachments); jsonErr != nil {
		t.Fatalf("parsing --json output: %v\n%s", jsonErr, stdout)
	}
	if len(attachments) != 1 {
		t.Fatalf("got %d attachments on stdout, want only the one that succeeded:\n%s",
			len(attachments), stdout)
	}
	if attachments[0].Name != "small.txt" {
		t.Errorf("name = %q, want %q", attachments[0].Name, "small.txt")
	}
	if attachments[0].DownloadURL == "" {
		t.Error("browser_download_url is empty")
	}

	if !strings.Contains(stderr, "still attached") {
		t.Errorf("expected stderr to say the uploaded file stays attached:\n%s", stderr)
	}
	if !strings.Contains(stderr, "oversized.bin") {
		t.Errorf("expected stderr to name the file that failed:\n%s", stderr)
	}

	// The attachment fj reported is real, not just an object it printed.
	if got := fetchAttachment(t, attachments[0].DownloadURL); string(got) != "this one lands\n" {
		t.Errorf("downloaded content = %q, want %q", got, "this one lands\n")
	}
}
