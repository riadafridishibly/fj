package auth

import (
	"os"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

func factoryWithHosts(hosts ...string) (*cmdutil.Factory, *config.Config) {
	cfg := &config.Config{Hosts: map[string]*config.HostConfig{}}
	for _, h := range hosts {
		cfg.Hosts[h] = &config.HostConfig{Token: "t-" + h, User: "me"}
	}
	return &cmdutil.Factory{Config: func() (*config.Config, error) { return cfg, nil }}, cfg
}

// TestLogoutHost pins how logout picks the host: --hostname or FJ_HOST,
// otherwise the only configured host. A failed pick leaves the config file
// alone.
func TestLogoutHost(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		fjHost  string
		wantErr string
	}{
		{"several hosts need --hostname", []string{"a.example.com", "b.example.com"}, "", "pass --hostname"},
		{"FJ_HOST not configured", []string{"a.example.com"}, "b.example.com", "not logged in to b.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FJ_CONFIG_DIR", t.TempDir())
			t.Setenv("FJ_HOST", tt.fjHost)
			f, cfg := factoryWithHosts(tt.hosts...)

			err := logoutRun(&logoutOptions{Factory: f})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if len(cfg.Hosts) != len(tt.hosts) {
				t.Errorf("hosts = %d, want all %d kept", len(cfg.Hosts), len(tt.hosts))
			}
			if _, err := os.Stat(config.ConfigPath()); !os.IsNotExist(err) {
				t.Errorf("config file was written: %v", err)
			}
		})
	}

	t.Run("FJ_HOST picks one of several", func(t *testing.T) {
		t.Setenv("FJ_CONFIG_DIR", t.TempDir())
		t.Setenv("FJ_HOST", "b.example.com")
		f, cfg := factoryWithHosts("a.example.com", "b.example.com")

		if err := logoutRun(&logoutOptions{Factory: f}); err != nil {
			t.Fatal(err)
		}
		if _, ok := cfg.Hosts["a.example.com"]; !ok || len(cfg.Hosts) != 1 {
			t.Errorf("hosts = %v, want only a.example.com left", cfg.Hosts)
		}
	})
}
