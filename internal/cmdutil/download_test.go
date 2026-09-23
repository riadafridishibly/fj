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
// an outside server a release asset links to.
func TestDownloadFileToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte("data"))
	}))
	defer srv.Close()
	srvHost := strings.TrimPrefix(srv.URL, "http://")
	dest := filepath.Join(t.TempDir(), "asset")

	for _, tc := range []struct {
		name, host, want string
	}{
		{"repository host", srvHost, "token secret"},
		{"outside server", "forgejo.example.com", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := DownloadFile(srv.URL+"/asset", tc.host, "secret", dest); err != nil {
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
}
