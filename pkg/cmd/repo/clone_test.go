package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

// TestCloneWithSeveralHosts: with two hosts and no way to pick one, a URL
// still clones and a bad argument reports its format, not the host.
func TestCloneWithSeveralHosts(t *testing.T) {
	t.Setenv("FJ_HOST", "")
	cfg := &config.Config{Hosts: map[string]*config.HostConfig{
		"a.example.com": {Token: "a", User: "me"},
		"b.example.com": {Token: "b", User: "me"},
	}}
	f := &cmdutil.Factory{Config: func() (*config.Config, error) { return cfg, nil }}
	dir := t.TempDir()
	t.Chdir(dir)

	src := filepath.Join(dir, "src.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", src).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	dest := filepath.Join(dir, "dest")
	if err := cloneRun(&cloneOptions{Factory: f, Repo: "file://" + src, Directory: dest}); err != nil {
		t.Fatalf("clone of a URL: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Errorf("clone did not create %s: %v", dest, err)
	}

	err := cloneRun(&cloneOptions{Factory: f, Repo: "myrepo"})
	if err == nil || !strings.Contains(err.Error(), "expected OWNER/REPO") {
		t.Errorf("error = %v, want the OWNER/REPO format error", err)
	}
}
