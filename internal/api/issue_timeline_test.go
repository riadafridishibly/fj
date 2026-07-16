package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestListIssueTimeline covers decoding the fields fj renders and the path
// the client builds, including the null entries Forgejo emits for events it
// cannot load.
func TestListIssueTimeline(t *testing.T) {
	const body = `[
		{
			"id": 1,
			"type": "comment",
			"body": "a plain comment",
			"user": {"login": "riad"},
			"created_at": "2026-07-17T10:00:00Z"
		},
		null,
		{
			"id": 2,
			"type": "commit_ref",
			"user": {"login": "riad"},
			"ref_commit_sha": "0123456789abcdef",
			"ref_action": "closes",
			"created_at": "2026-07-17T12:00:00Z"
		}
	]`

	var gotPath, gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAuth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	events, err := client.ListIssueTimeline("owner", "repo", 42, ListIssueTimelineOptions{})
	if err != nil {
		t.Fatalf("ListIssueTimeline() error = %v", err)
	}

	if want := "/api/v1/repos/owner/repo/issues/42/timeline"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty when no options are set", gotQuery)
	}
	if want := "token sekrit"; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}

	// The null entry must not survive as a nil the caller would dereference.
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (the null dropped)", len(events))
	}
	for i, e := range events {
		if e == nil {
			t.Fatalf("events[%d] is nil", i)
		}
	}

	ref := events[1]
	if ref.Type != EventCommitRef {
		t.Errorf("Type = %q, want %q", ref.Type, EventCommitRef)
	}
	if ref.RefCommitSHA != "0123456789abcdef" {
		t.Errorf("RefCommitSHA = %q, want %q", ref.RefCommitSHA, "0123456789abcdef")
	}
	if ref.RefAction != RefActionCloses {
		t.Errorf("RefAction = %q, want %q", ref.RefAction, RefActionCloses)
	}
	if ref.Poster == nil || ref.Poster.UserName != "riad" {
		t.Errorf("Poster = %+v, want username riad", ref.Poster)
	}
	if want := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC); !ref.Created.Equal(want) {
		t.Errorf("Created = %v, want %v", ref.Created, want)
	}
}

func TestListIssueTimelineQueryEncode(t *testing.T) {
	opt := ListIssueTimelineOptions{Since: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}
	opt.Page, opt.PageSize = 2, 50

	want := "limit=50&page=2&since=2026-07-01T00%3A00%3A00Z"
	if got := opt.QueryEncode(); got != want {
		t.Errorf("QueryEncode() = %q, want %q", got, want)
	}
}

func TestListIssueTimelineError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"issue does not exist"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	if _, err := client.ListIssueTimeline("owner", "repo", 42, ListIssueTimelineOptions{}); err == nil {
		t.Fatal("ListIssueTimeline() error = nil, want an error carrying the server message")
	}
}
