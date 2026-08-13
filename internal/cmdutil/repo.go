package cmdutil

import (
	"fmt"
	"slices"
	"strings"

	"github.com/riadafridishibly/fj/internal/config"
	"github.com/riadafridishibly/fj/internal/git"
)

type Repo struct {
	Host  string
	Owner string
	Name  string
}

func (r Repo) FullName() string {
	return r.Owner + "/" + r.Name
}

// RepoFromFullName parses a repository selector in gh's [HOST/]OWNER/REPO
// form. The two-segment form leaves Host empty for the caller to fill in
// from configuration.
func RepoFromFullName(name string) (Repo, error) {
	parts := strings.Split(name, "/")
	if slices.Contains(parts, "") {
		return Repo{}, invalidRepoName(name)
	}
	switch len(parts) {
	case 2:
		return Repo{Owner: parts[0], Name: parts[1]}, nil
	case 3:
		return Repo{Host: parts[0], Owner: parts[1], Name: parts[2]}, nil
	}
	return Repo{}, invalidRepoName(name)
}

func invalidRepoName(name string) error {
	return fmt.Errorf("expected [HOST/]OWNER/REPO format, got %q", name)
}

// RepoFromGitRemotes resolves the repository from git remotes by matching
// against configured hosts. Prefers "origin" remote.
func RepoFromGitRemotes(cfg *config.Config) (Repo, error) {
	remotes, err := git.Remotes()
	if err != nil {
		return Repo{}, fmt.Errorf("failed to get git remotes: %w", err)
	}
	if len(remotes) == 0 {
		return Repo{}, fmt.Errorf("no git remotes found. Use -R OWNER/REPO to specify a repository")
	}

	// Try origin first, then any remote
	var candidates []git.Remote
	for _, r := range remotes {
		if r.Name == "origin" {
			candidates = append([]git.Remote{r}, candidates...)
		} else {
			candidates = append(candidates, r)
		}
	}

	for _, remote := range candidates {
		rawURL := remote.FetchURL
		if rawURL == "" {
			rawURL = remote.PushURL
		}
		info, err := git.ParseRemoteURL(rawURL)
		if err != nil {
			continue
		}
		// Check if this host is in our config
		if _, ok := cfg.Hosts[info.Host]; ok {
			return Repo{Host: info.Host, Owner: info.Owner, Name: info.Name}, nil
		}
	}

	// If only one host configured, use first parseable remote
	if len(cfg.Hosts) == 1 {
		var host string
		for h := range cfg.Hosts {
			host = h
		}
		for _, remote := range candidates {
			rawURL := remote.FetchURL
			if rawURL == "" {
				rawURL = remote.PushURL
			}
			info, err := git.ParseRemoteURL(rawURL)
			if err != nil {
				continue
			}
			return Repo{Host: host, Owner: info.Owner, Name: info.Name}, nil
		}
	}

	return Repo{}, fmt.Errorf("could not determine repository from git remotes. Use -R OWNER/REPO to specify")
}
