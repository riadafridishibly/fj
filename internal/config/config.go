package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// JSON Schema for config validation by editors
const SchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "fj configuration",
  "description": "Configuration file for the fj Forgejo CLI",
  "type": "object",
  "properties": {
    "default_host": {
      "type": "string",
      "description": "Host to use when a command does not name one; must match a key under hosts"
    },
    "hosts": {
      "type": "object",
      "description": "Forgejo host configurations",
      "additionalProperties": {
        "type": "object",
        "properties": {
          "hostname": {
            "type": "string",
            "description": "The hostname of the Forgejo instance"
          },
          "token": {
            "type": "string",
            "description": "API access token"
          },
          "user": {
            "type": "string",
            "description": "Username on this host"
          },
          "git_protocol": {
            "type": "string",
            "enum": ["https", "ssh"],
            "default": "https",
            "description": "Git protocol to use (https or ssh)"
          }
        },
        "required": ["token", "user"],
        "additionalProperties": false
      }
    }
  },
  "required": ["hosts"],
  "additionalProperties": false
}`

type HostConfig struct {
	Hostname    string `yaml:"hostname" json:"hostname"`
	Token       string `yaml:"token" json:"token"`
	User        string `yaml:"user" json:"user"`
	GitProtocol string `yaml:"git_protocol" json:"git_protocol"`
}

type Config struct {
	// DefaultHostName names the entry in Hosts that commands target when
	// neither a selector nor a flag picks one. Optional with a single host;
	// required to disambiguate multiple hosts.
	DefaultHostName string                 `yaml:"default_host,omitempty" json:"default_host,omitempty"`
	Hosts           map[string]*HostConfig `yaml:"hosts" json:"hosts"`
}

func ConfigDir() string {
	if d := os.Getenv("FJ_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserConfigDir()
	if err != nil {
		home, _ = os.UserHomeDir()
		return filepath.Join(home, ".config", "fj")
	}
	return filepath.Join(home, "fj")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func SchemaPath() string {
	return filepath.Join(ConfigDir(), "schema.json")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Hosts: make(map[string]*HostConfig)}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", ConfigPath(), err)
	}
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]*HostConfig)
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("# fj configuration file\n# yaml-language-server: $schema=%s\n", SchemaPath())
	if err := os.WriteFile(ConfigPath(), append([]byte(header), data...), 0o600); err != nil {
		return err
	}
	// Also write the JSON schema for editor support
	return WriteSchema()
}

// DefaultHost returns the host commands should target when nothing else
// selects one: the host named by the default_host config key, or the sole
// configured host. With several hosts and no default_host the choice is
// ambiguous and an error.
func (c *Config) DefaultHost() (*HostConfig, string, error) {
	if len(c.Hosts) == 0 {
		return nil, "", fmt.Errorf("not logged in to any host. Run 'fj auth login' to authenticate")
	}
	if c.DefaultHostName != "" {
		h, ok := c.Hosts[c.DefaultHostName]
		if !ok {
			return nil, "", fmt.Errorf("default_host %q is not a configured host (configured: %s)",
				c.DefaultHostName, strings.Join(c.hostNames(), ", "))
		}
		return h, c.DefaultHostName, nil
	}
	if names := c.hostNames(); len(names) > 1 {
		return nil, "", fmt.Errorf(
			"multiple hosts configured (%s); pass HOST/OWNER/REPO or set default_host in %s",
			strings.Join(names, ", "), ConfigPath())
	}
	for name, h := range c.Hosts {
		return h, name, nil
	}
	return nil, "", nil // unreachable
}

func (c *Config) hostNames() []string {
	return slices.Sorted(maps.Keys(c.Hosts))
}

func (c *Config) HostByName(name string) (*HostConfig, error) {
	h, ok := c.Hosts[name]
	if !ok {
		return nil, fmt.Errorf("not logged in to %s. Run 'fj auth login --hostname %s'", name, name)
	}
	return h, nil
}

func (c *Config) TokenForHost(hostname string) (string, error) {
	if t := os.Getenv("FJ_TOKEN"); t != "" {
		return t, nil
	}
	h, err := c.HostByName(hostname)
	if err != nil {
		return "", err
	}
	return h.Token, nil
}

func WriteSchema() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(SchemaPath(), []byte(SchemaJSON), 0o644)
}
