package comment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestListAllReviewsCappedPages(t *testing.T) {
	for _, count := range []int{0, 10, 23} {
		for _, withTotal := range []bool{false, true} {
			t.Run(strconv.Itoa(count)+"/total="+strconv.FormatBool(withTotal), func(t *testing.T) {
				requests := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// SDK client creation may request the server version.
					if r.URL.Path == "/api/v1/version" {
						_, _ = w.Write([]byte(`{"version":"14.0.0"}`))
						return
					}
					if withTotal {
						w.Header().Set("X-Total-Count", strconv.Itoa(count))
					}
					requests++
					if r.Method != http.MethodGet || r.URL.Path != "/api/v1/repos/owner/repo/pulls/7/reviews" || r.URL.Query().Get("limit") != "50" {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					}
					if r.URL.Query().Get("page") != strconv.Itoa(requests) {
						t.Errorf("page = %q, want %d", r.URL.Query().Get("page"), requests)
					}
					// Forgejo caps the response at 10, despite the requested 50.
					var reviews []*forgejo.PullReview
					for i := (requests-1)*10 + 1; i <= min(requests*10, count); i++ {
						reviews = append(reviews, &forgejo.PullReview{ID: int64(i)})
					}
					_ = json.NewEncoder(w).Encode(reviews)
				}))
				defer srv.Close()
				client, err := forgejo.NewClient(srv.URL, forgejo.SetHTTPClient(srv.Client()))
				if err != nil {
					t.Fatal(err)
				}
				got, err := listAllReviews(client, "owner", "repo", 7)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != count {
					t.Fatalf("got %d reviews, want %d", len(got), count)
				}
				for i, review := range got {
					if review.ID != int64(i+1) {
						t.Errorf("review %d ID = %d, want %d", i, review.ID, i+1)
					}
				}
				wantRequests := (count+9)/10 + 1
				if withTotal && count > 0 {
					wantRequests--
				}
				if requests != wantRequests {
					t.Errorf("requests = %d, want %d", requests, wantRequests)
				}
			})
		}
	}
}

func TestListAllReviewsPageError(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/version" {
					_, _ = w.Write([]byte(`{"version":"14.0.0"}`))
					return
				}
				requests++
				if requests == 1 {
					_, _ = w.Write([]byte(`[{"id":1}]`))
					return
				}
				http.Error(w, `{"message":"listing failed"}`, status)
			}))
			defer srv.Close()
			client, err := forgejo.NewClient(srv.URL, forgejo.SetHTTPClient(srv.Client()))
			if err != nil {
				t.Fatal(err)
			}
			got, err := listAllReviews(client, "owner", "repo", 7)
			if err == nil || got != nil {
				t.Fatalf("listAllReviews = %v, %v; want no partial result and an error", got, err)
			}
			if status == http.StatusNotFound && !strings.Contains(err.Error(), "pr #7 not found in owner/repo") {
				t.Errorf("unexpected not-found error: %v", err)
			}
		})
	}
}
