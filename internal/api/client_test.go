package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// request captures what the server saw, for the header and path assertions
// the JSON-bodied methods care about.
type request struct {
	method      string
	path        string
	rawQuery    string
	contentType string
	body        string
}

func recordingServer(t *testing.T, got *request, status int, response string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.rawQuery = r.URL.RawQuery
		got.contentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading body: %v", err)
		}
		got.body = string(body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if response != "" {
			w.Write([]byte(response))
		}
	}))
}

// TestJSONRequestSetsContentType pins the one header a JSON-bodied request
// must carry. newRequest decides it from a content type string rather than
// from the body, so nothing else in the package proves it is still set.
func TestJSONRequestSetsContentType(t *testing.T) {
	var got request
	srv := recordingServer(t, &got, http.StatusCreated, `{"id":3,"body":"looks wrong"}`)
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	comment, err := client.CreatePullReviewComment("owner", "repo", 7, 11,
		forgejo.CreatePullReviewComment{Path: "main.go", Body: "looks wrong"})
	if err != nil {
		t.Fatalf("CreatePullReviewComment() error = %v", err)
	}

	if want := "application/json"; got.contentType != want {
		t.Errorf("Content-Type = %q, want %q", got.contentType, want)
	}
	if want := "/api/v1/repos/owner/repo/pulls/7/reviews/11/comments"; got.path != want {
		t.Errorf("path = %q, want %q", got.path, want)
	}
	if got.rawQuery != "" {
		t.Errorf("query = %q, want it empty", got.rawQuery)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(got.body), &payload); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, got.body)
	}
	if payload["body"] != "looks wrong" {
		t.Errorf("body field = %v, want %q", payload["body"], "looks wrong")
	}
	if comment.ID != 3 {
		t.Errorf("ID = %d, want 3", comment.ID)
	}
}

// TestBodilessRequestSetsNoContentType is the other half of that decision: a
// request with no payload must not claim one.
func TestBodilessRequestSetsNoContentType(t *testing.T) {
	var got request
	srv := recordingServer(t, &got, http.StatusNoContent, "")
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	if err := client.DeletePullReviewComment("owner", "repo", 7, 11, 3); err != nil {
		t.Fatalf("DeletePullReviewComment() error = %v", err)
	}

	if got.contentType != "" {
		t.Errorf("Content-Type = %q, want it unset", got.contentType)
	}
	if got.method != http.MethodDelete {
		t.Errorf("method = %q, want %q", got.method, http.MethodDelete)
	}
	if want := "/api/v1/repos/owner/repo/pulls/7/reviews/11/comments/3"; got.path != want {
		t.Errorf("path = %q, want %q", got.path, want)
	}
}

// TestRepoPathEscapesSegments covers the reason repoPath exists: an owner or
// repo carrying URL syntax must stay one path segment instead of splitting
// the request off to another endpoint.
func TestRepoPathEscapesSegments(t *testing.T) {
	tests := []struct {
		name  string
		owner string
		repo  string
		want  string
	}{
		{"plain", "owner", "repo", "/repos/owner/repo/issues/42/assets"},
		{"query", "owner", "repo?x=1", "/repos/owner/repo%3Fx=1/issues/42/assets"},
		{"fragment", "owner", "repo#frag", "/repos/owner/repo%23frag/issues/42/assets"},
		{"dot segments", "owner", "../../admin/users", "/repos/owner/..%2F..%2Fadmin%2Fusers/issues/42/assets"},
		{"slash in owner", "a/b", "repo", "/repos/a%2Fb/repo/issues/42/assets"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repoPath(tt.owner, tt.repo, "/issues/%d/assets", 42); got != tt.want {
				t.Errorf("repoPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestUploadPathSurvivesHostileRepoName is the end-to-end version: the server
// must see the assets endpoint, with nothing pushed into the query string.
func TestUploadPathSurvivesHostileRepoName(t *testing.T) {
	var got upload
	srv := attachmentServer(t, &got)
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	if _, err := client.CreateIssueAttachment("owner", "repo?x=1", 42,
		strings.NewReader("bytes"), "login.png"); err != nil {
		t.Fatalf("CreateIssueAttachment() error = %v", err)
	}

	if want := "/api/v1/repos/owner/repo?x=1/issues/42/assets"; got.path != want {
		t.Errorf("path = %q, want %q", got.path, want)
	}
}
