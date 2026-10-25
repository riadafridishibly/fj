package cmdutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/riadafridishibly/fj/internal/api"
)

func TestListAllCappedPages(t *testing.T) {
	for _, count := range []int{0, 10, 23} {
		for _, withTotal := range []bool{false, true} {
			t.Run(strconv.Itoa(count)+"/total="+strconv.FormatBool(withTotal), func(t *testing.T) {
				var want []int
				for i := 1; i <= count; i++ {
					want = append(want, i)
				}
				requests := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if withTotal {
						w.Header().Set("X-Total-Count", strconv.Itoa(count))
					}
					requests++
					if r.Method != http.MethodGet || r.URL.Path != "/api/v1/items" || r.URL.Query().Get("state") != "open" || r.URL.Query().Get("limit") != "50" {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					}
					if r.URL.Query().Get("page") != strconv.Itoa(requests) {
						t.Errorf("page = %q, want %d", r.URL.Query().Get("page"), requests)
					}
					// Simulate MAX_RESPONSE_ITEMS=10, with and without a total count.
					start := min((requests-1)*10, count)
					end := min(start+10, count)
					_ = json.NewEncoder(w).Encode(want[start:end])
				}))
				defer srv.Close()

				got, err := ListAll[int](api.NewClient(srv.URL, "", srv.Client()), "/items?state=open")
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, want) {
					t.Fatalf("items = %v, want %v", got, want)
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

func TestListAllPageError(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			_, _ = w.Write([]byte(`[1]`))
			return
		}
		http.Error(w, `{"message":"listing failed"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	got, err := ListAll[int](api.NewClient(srv.URL, "", srv.Client()), "/items")
	if err == nil || got != nil {
		t.Fatalf("ListAll = %v, %v; want no partial result and an error", got, err)
	}
}
