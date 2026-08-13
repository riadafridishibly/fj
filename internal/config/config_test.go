package config

import (
	"strings"
	"testing"
)

func TestDefaultHost(t *testing.T) {
	a := &HostConfig{Hostname: "a.example.com", Token: "ta", User: "u"}
	b := &HostConfig{Hostname: "b.example.com", Token: "tb", User: "u"}

	t.Run("no hosts", func(t *testing.T) {
		c := &Config{Hosts: map[string]*HostConfig{}}
		if _, _, err := c.DefaultHost(); err == nil {
			t.Fatal("want error for empty config")
		}
	})

	t.Run("single host", func(t *testing.T) {
		c := &Config{Hosts: map[string]*HostConfig{"a.example.com": a}}
		h, name, err := c.DefaultHost()
		if err != nil {
			t.Fatal(err)
		}
		if name != "a.example.com" || h != a {
			t.Errorf("got %q, want a.example.com", name)
		}
	})

	t.Run("multiple hosts without default is ambiguous", func(t *testing.T) {
		c := &Config{Hosts: map[string]*HostConfig{"a.example.com": a, "b.example.com": b}}
		_, _, err := c.DefaultHost()
		if err == nil {
			t.Fatal("want error for ambiguous hosts")
		}
		for _, want := range []string{"a.example.com", "b.example.com", "default_host"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not mention %q", err, want)
			}
		}
	})

	t.Run("default_host selects among multiple", func(t *testing.T) {
		c := &Config{
			DefaultHostName: "b.example.com",
			Hosts:           map[string]*HostConfig{"a.example.com": a, "b.example.com": b},
		}
		h, name, err := c.DefaultHost()
		if err != nil {
			t.Fatal(err)
		}
		if name != "b.example.com" || h != b {
			t.Errorf("got %q, want b.example.com", name)
		}
	})

	t.Run("default_host must be configured", func(t *testing.T) {
		c := &Config{
			DefaultHostName: "missing.example.com",
			Hosts:           map[string]*HostConfig{"a.example.com": a},
		}
		_, _, err := c.DefaultHost()
		if err == nil || !strings.Contains(err.Error(), "missing.example.com") {
			t.Errorf("got %v, want error naming the missing host", err)
		}
	})
}

func TestDefaultHostRoundTrip(t *testing.T) {
	t.Setenv("FJ_CONFIG_DIR", t.TempDir())

	saved := &Config{
		DefaultHostName: "b.example.com",
		Hosts: map[string]*HostConfig{
			"a.example.com": {Hostname: "a.example.com", Token: "ta", User: "u"},
			"b.example.com": {Hostname: "b.example.com", Token: "tb", User: "u"},
		},
	}
	if err := saved.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultHostName != "b.example.com" {
		t.Errorf("DefaultHostName = %q, want b.example.com", loaded.DefaultHostName)
	}
	_, name, err := loaded.DefaultHost()
	if err != nil {
		t.Fatal(err)
	}
	if name != "b.example.com" {
		t.Errorf("DefaultHost = %q, want b.example.com", name)
	}
}
