package cmdutil

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDownloadFileToken: the token goes to the repository's host and not to
// an outside server, over plain http without FJ_INSECURE, or through a
// redirect that leaves the host.
func TestDownloadFileToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte("data"))
	}))
	defer srv.Close()
	// Same hostname, another port: Go would keep the header on its own.
	redir := httptest.NewServer(http.RedirectHandler(srv.URL+"/asset", http.StatusFound))
	defer redir.Close()
	host := func(s *httptest.Server) string { return strings.TrimPrefix(s.URL, "http://") }
	dest := filepath.Join(t.TempDir(), "asset")

	for _, tc := range []struct {
		name, url, host, insecure, want string
	}{
		{"repository host", srv.URL + "/asset", host(srv), "1", "token secret"},
		{"outside server", srv.URL + "/asset", "forgejo.example.com", "1", ""},
		{"plain http without FJ_INSECURE", srv.URL + "/asset", host(srv), "", ""},
		{"redirect off the host", redir.URL + "/asset", host(redir), "1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FJ_INSECURE", tc.insecure)
			gotAuth = "<no request>"
			if err := DownloadFile(tc.url, tc.host, "secret", dest); err != nil {
				t.Fatal(err)
			}
			if gotAuth != tc.want {
				t.Errorf("Authorization = %q, want %q", gotAuth, tc.want)
			}
			if b, _ := os.ReadFile(dest); string(b) != "data" {
				t.Errorf("file = %q, want data", b)
			}
		})
	}

	t.Run("a cut-off body leaves no file", func(t *testing.T) {
		short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "100")
			w.Write([]byte("data"))
		}))
		defer short.Close()
		dest := filepath.Join(t.TempDir(), "partial")
		if err := DownloadFile(short.URL, host(short), "secret", dest); err == nil {
			t.Fatal("want an error for a cut-off body")
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Errorf("partial file left at %s: %v", dest, err)
		}
	})
}

// TestAPIClientRedirect: the API client drops the token on a redirect off
// the host, as DownloadFile does.
func TestAPIClientRedirect(t *testing.T) {
	gotAuth := "<no request>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer srv.Close()
	// Same hostname, another port: Go would keep the header on its own.
	redir := httptest.NewServer(http.RedirectHandler(srv.URL+"/x", http.StatusFound))
	defer redir.Close()
	t.Setenv("FJ_INSECURE", "1")
	t.Setenv("FJ_TOKEN", "")
	t.Setenv("FJ_HOST", "")

	host := strings.TrimPrefix(redir.URL, "http://")
	client, err := factoryWithHosts(host).APIClientForHost(host)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotAuth != "" {
		t.Errorf("Authorization after the redirect = %q, want none", gotAuth)
	}
}
