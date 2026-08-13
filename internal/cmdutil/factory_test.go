package cmdutil

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/config"
)

// twoHostFactory returns a factory configured for two hosts, so nothing but
// the resolution order can pick one.
func twoHostFactory(defaultHost string) *Factory {
	cfg := &config.Config{
		DefaultHostName: defaultHost,
		Hosts: map[string]*config.HostConfig{
			"a.example.com": {Hostname: "a.example.com", Token: "ta", User: "u"},
			"b.example.com": {Hostname: "b.example.com", Token: "tb", User: "u"},
		},
	}
	return &Factory{Config: func() (*config.Config, error) { return cfg, nil }}
}

// chdirGitRepo moves the test into a throwaway repository whose only remote
// points at host. Nothing is fetched: repo resolution only reads the URL.
func chdirGitRepo(t *testing.T, host string) {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"remote", "add", "origin", "https://" + host + "/someone/their-repo.git"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	t.Chdir(dir)
}

func TestRepoFromArg(t *testing.T) {
	t.Run("explicit host wins", func(t *testing.T) {
		chdirGitRepo(t, "b.example.com")
		repo, err := twoHostFactory("a.example.com").RepoFromArg("a.example.com/example-org/example-repo")
		if err != nil {
			t.Fatal(err)
		}
		if repo.Host != "a.example.com" {
			t.Errorf("host = %q, want a.example.com", repo.Host)
		}
	})

	t.Run("current clone beats default_host", func(t *testing.T) {
		chdirGitRepo(t, "b.example.com")
		repo, err := twoHostFactory("a.example.com").RepoFromArg("example-org/example-repo")
		if err != nil {
			t.Fatal(err)
		}
		want := Repo{Host: "b.example.com", Owner: "example-org", Name: "example-repo"}
		if repo != want {
			t.Errorf("repo = %+v, want %+v", repo, want)
		}
	})

	t.Run("default_host used outside a git repository", func(t *testing.T) {
		t.Chdir(t.TempDir())
		repo, err := twoHostFactory("a.example.com").RepoFromArg("example-org/example-repo")
		if err != nil {
			t.Fatal(err)
		}
		if repo.Host != "a.example.com" {
			t.Errorf("host = %q, want a.example.com", repo.Host)
		}
	})

	t.Run("ambiguous without a clone or a default", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := twoHostFactory("").RepoFromArg("example-org/example-repo")
		if err == nil {
			t.Fatal("want an error naming the configured hosts")
		}
		if !strings.Contains(err.Error(), "HOST/OWNER/REPO") {
			t.Errorf("error = %q, want it to point at the HOST/OWNER/REPO form", err)
		}
	})
}
