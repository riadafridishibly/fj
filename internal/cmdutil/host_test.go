package cmdutil

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/config"
)

func factoryWithHosts(hosts ...string) *Factory {
	cfg := &config.Config{Hosts: map[string]*config.HostConfig{}}
	for _, h := range hosts {
		cfg.Hosts[h] = &config.HostConfig{Hostname: h, Token: "t-" + h, User: "me"}
	}
	return &Factory{Config: func() (*config.Config, error) { return cfg, nil }}
}

// checkout makes the test's working directory a git repository with the
// given remotes, name then URL.
func checkout(t *testing.T, remotes ...string) {
	t.Helper()
	t.Chdir(t.TempDir())
	run := func(args ...string) {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	for i := 0; i < len(remotes); i += 2 {
		run("remote", "add", remotes[i], remotes[i+1])
	}
}

func TestHost(t *testing.T) {
	t.Setenv("FJ_HOST", "")

	t.Run("several hosts and nothing to choose is an error", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := factoryWithHosts("b.example.com", "a.example.com").Host()
		want := "multiple hosts configured (a.example.com, b.example.com); set FJ_HOST to choose one"
		if err == nil || err.Error() != want {
			t.Errorf("error = %v, want %q", err, want)
		}
	})

	t.Run("no hosts is an error", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, err := factoryWithHosts().Host(); err == nil || !strings.Contains(err.Error(), "not logged in") {
			t.Errorf("error = %v, want not logged in", err)
		}
	})

	t.Run("one host", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if got, err := factoryWithHosts("a.example.com").Host(); err != nil || got != "a.example.com" {
			t.Errorf("Host() = %q, %v", got, err)
		}
	})

	t.Run("FJ_HOST, then --hostname over it", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("FJ_HOST", "b.example.com")
		f := factoryWithHosts("a.example.com", "b.example.com")
		if got, err := f.Host(); err != nil || got != "b.example.com" {
			t.Errorf("FJ_HOST: Host() = %q, %v", got, err)
		}
		f.HostOverride = "a.example.com"
		if got, err := f.Host(); err != nil || got != "a.example.com" {
			t.Errorf("--hostname: Host() = %q, %v", got, err)
		}
	})

	t.Run("the checkout's host", func(t *testing.T) {
		checkout(t, "origin", "https://b.example.com/o/r.git")
		if got, err := factoryWithHosts("a.example.com", "b.example.com").Host(); err != nil || got != "b.example.com" {
			t.Errorf("Host() = %q, %v", got, err)
		}
	})

	// Without an origin, the remote used to come from map order. Two
	// remotes on different hosts must give the same answer on every run.
	t.Run("the same host on every run without an origin", func(t *testing.T) {
		checkout(t,
			"beta", "https://a.example.com/o/r.git",
			"alpha", "https://b.example.com/o/r.git")
		f := factoryWithHosts("a.example.com", "b.example.com")
		for range 20 {
			if got, err := f.Host(); err != nil || got != "b.example.com" {
				t.Fatalf("Host() = %q, %v; want the first remote by name", got, err)
			}
		}
	})
}
