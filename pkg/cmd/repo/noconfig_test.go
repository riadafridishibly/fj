package repo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

// TestWithoutConfig covers the CI pairing: with FJ_HOST and FJ_TOKEN and no
// config file, repo clone and repo list work, and list names the user from
// the API since there is no stored one.
func TestWithoutConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	src := filepath.Join(dir, "src.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", src).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	repo := map[string]any{"name": "r", "full_name": "o/r", "clone_url": "file://" + src}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body any
		switch r.URL.Path {
		case "/api/v1/version":
			body = map[string]string{"version": "14.0.0"}
		case "/api/v1/repos/o/r":
			body = repo
		case "/api/v1/user":
			body = map[string]string{"login": "ci-bot"}
		case "/api/v1/user/repos":
			body = []any{}
			if r.URL.Query().Get("page") == "1" {
				body = []any{repo}
			}
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	t.Setenv("FJ_INSECURE", "1")
	t.Setenv("FJ_HOST", strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("FJ_TOKEN", "env")
	cfg := &config.Config{Hosts: map[string]*config.HostConfig{}}
	f := &cmdutil.Factory{Config: func() (*config.Config, error) { return cfg, nil }}

	dest := filepath.Join(dir, "dest")
	if err := cloneRun(&cloneOptions{Factory: f, Repo: "o/r", Directory: dest}); err != nil {
		t.Fatalf("repo clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Errorf("clone did not create %s: %v", dest, err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = listRun(&listOptions{Factory: f, Limit: 30})
	w.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatalf("repo list: %v", err)
	}
	if out, _ := io.ReadAll(r); !strings.Contains(string(out), "in @ci-bot") {
		t.Errorf("repo list output does not name the user:\n%s", out)
	}
}
