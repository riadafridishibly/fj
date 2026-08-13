package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// issuePage renders n issues numbered from start — enough for the paging
// loop to count and for a short page to end it.
func issuePage(start, n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = fmt.Sprintf(`{"number": %d}`, start+i)
	}
	return "[" + strings.Join(items, ",") + "]"
}

func TestSearchIssuesPagesToTheEnd(t *testing.T) {
	// Two full pages then a short one. Stopping after the first would drop
	// matches that, with no repository filter on the endpoint, routinely sit
	// past it.
	pages := []string{issuePage(1, 2), issuePage(3, 2), issuePage(5, 1)}

	var gotPages []string
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPages = append(gotPages, r.URL.Query().Get("page"))
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 || page > len(pages) {
			fmt.Fprint(w, "[]")
			return
		}
		fmt.Fprint(w, pages[page-1])
	}))
	defer srv.Close()

	opt := SearchIssuesOptions{Type: "pulls", State: "open", ReviewRequested: true}
	opt.PageSize = 2

	issues, err := NewClient(srv.URL, "sekrit", srv.Client()).SearchIssues(opt)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}

	if len(issues) != 5 {
		t.Errorf("got %d issues, want 5 across three pages", len(issues))
	}
	if want := []string{"1", "2", "3"}; !slices.Equal(gotPages, want) {
		t.Errorf("requested pages = %v, want %v (stopping on the short page)", gotPages, want)
	}
	if !strings.Contains(gotQuery, "review_requested=true") {
		t.Errorf("query = %q, want it to carry review_requested=true", gotQuery)
	}
}

// The endpoint answers "everything you were asked to review", across every
// repository, so the loop needs a stop of its own.
func TestSearchIssuesStopsAtThePageCap(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, issuePage(requests, 2)) // never a short page
	}))
	defer srv.Close()

	opt := SearchIssuesOptions{}
	opt.PageSize = 2

	if _, err := NewClient(srv.URL, "sekrit", srv.Client()).SearchIssues(opt); err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if requests != searchIssuesMaxPages {
		t.Errorf("made %d requests, want the loop bounded at %d", requests, searchIssuesMaxPages)
	}
}
