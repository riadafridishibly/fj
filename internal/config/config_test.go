package config

import (
	"strings"
	"testing"
)

// TestTokenForHostScopesFJToken pins which host FJ_TOKEN may reach: the
// FJ_HOST host, else the single configured host, else any host when none
// are configured. Several hosts without FJ_HOST is an error, never a guess.
func TestTokenForHostScopesFJToken(t *testing.T) {
	hosts := func(names ...string) *Config {
		c := &Config{Hosts: map[string]*HostConfig{}}
		for _, n := range names {
			c.Hosts[n] = &HostConfig{Token: "stored-" + n}
		}
		return c
	}

	for _, tc := range []struct {
		name    string
		cfg     *Config
		envHost string
		target  string
		want    string // token, or an error substring prefixed with "error: "
	}{
		{"FJ_HOST: its host gets FJ_TOKEN", hosts("a.example.com", "b.example.com"), "a.example.com", "a.example.com", "env"},
		{"FJ_HOST: other hosts keep their own", hosts("a.example.com", "b.example.com"), "a.example.com", "b.example.com", "stored-b.example.com"},
		{"FJ_HOST: unknown other host", hosts("a.example.com"), "ci.example.com", "b.example.com", "error: not logged in to b.example.com"},
		{"FJ_HOST without config", hosts(), "ci.example.com", "ci.example.com", "env"},
		{"no config: any host", hosts(), "", "ci.example.com", "env"},
		{"one host: that host", hosts("a.example.com"), "", "a.example.com", "env"},
		{"one host: not another", hosts("a.example.com"), "", "b.example.com", "error: not logged in to b.example.com"},
		{"several hosts: error", hosts("a.example.com", "b.example.com"), "", "a.example.com", "error: set FJ_HOST"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FJ_TOKEN", "env")
			t.Setenv("FJ_HOST", tc.envHost)

			got, err := tc.cfg.TokenForHost(tc.target)
			if want, ok := strings.CutPrefix(tc.want, "error: "); ok {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("TokenForHost(%q) = %q, %v; want error containing %q", tc.target, got, err, want)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("TokenForHost(%q) = %q, %v; want %q", tc.target, got, err, tc.want)
			}
		})
	}

	t.Run("without FJ_TOKEN, stored tokens", func(t *testing.T) {
		t.Setenv("FJ_TOKEN", "")
		t.Setenv("FJ_HOST", "")
		got, err := hosts("a.example.com", "b.example.com").TokenForHost("b.example.com")
		if err != nil || got != "stored-b.example.com" {
			t.Fatalf("TokenForHost = %q, %v", got, err)
		}
	})
}
