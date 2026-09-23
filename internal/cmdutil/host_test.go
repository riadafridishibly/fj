package cmdutil

import (
	"net/http"
	"net/http/httptest"
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

	t.Run("several hosts without FJ_HOST or a checkout is an error", func(t *testing.T) {
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

	t.Run("the checkout's host with a port", func(t *testing.T) {
		checkout(t, "origin", "http://localhost:3000/o/r.git")
		if got, err := factoryWithHosts("localhost:3000", "b.example.com").Host(); err != nil || got != "localhost:3000" {
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

// TestClientWithoutConfig covers the CI pairing: FJ_HOST and FJ_TOKEN with
// no config file must be enough to build a client.
func TestClientWithoutConfig(t *testing.T) {
	// The SDK asks the server for its version when the client is built.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "token env" {
			t.Errorf("Authorization = %q, want FJ_TOKEN", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"14.0.0"}`))
	}))
	defer srv.Close()
	ciHost := strings.TrimPrefix(srv.URL, "http://")

	t.Setenv("FJ_INSECURE", "1")
	t.Setenv("FJ_HOST", ciHost)
	t.Setenv("FJ_TOKEN", "env")

	f := factoryWithHosts()
	host, err := f.Host()
	if err != nil || host != ciHost {
		t.Fatalf("Host() = %q, %v", host, err)
	}
	if _, err := f.Client(host); err != nil {
		t.Fatalf("Client(%q) error = %v", host, err)
	}
}
