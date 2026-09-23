package auth

import (
	"os"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

// TestLogoutNeedsHostnameWithSeveralHosts pins the fix for logging out of a
// random host: without --hostname and with two hosts, logout fails and
// leaves the config file alone. It does not fall back to FJ_HOST or the
// checkout, since removing credentials should not be inferred.
func TestLogoutNeedsHostnameWithSeveralHosts(t *testing.T) {
	t.Setenv("FJ_CONFIG_DIR", t.TempDir())
	t.Setenv("FJ_HOST", "a.example.com")
	cfg := &config.Config{Hosts: map[string]*config.HostConfig{
		"a.example.com": {Token: "a", User: "me"},
		"b.example.com": {Token: "b", User: "me"},
	}}
	f := &cmdutil.Factory{Config: func() (*config.Config, error) { return cfg, nil }}

	err := logoutRun(&logoutOptions{Factory: f})
	if err == nil || !strings.Contains(err.Error(), "pass --hostname") {
		t.Fatalf("error = %v, want one asking for --hostname", err)
	}
	if len(cfg.Hosts) != 2 {
		t.Errorf("hosts = %d, want both kept", len(cfg.Hosts))
	}
	if _, err := os.Stat(config.ConfigPath()); !os.IsNotExist(err) {
		t.Errorf("config file was written: %v", err)
	}
}
