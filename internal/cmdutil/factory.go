package cmdutil

import (
	"fmt"
	"strconv"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/riadafridishibly/fj/internal/config"
)

type Factory struct {
	Config func() (*config.Config, error)

	// RepoOverride is set by the -R flag
	RepoOverride string

	// HostOverride is set by the --hostname flag on some commands
	HostOverride string
}

func NewFactory() *Factory {
	var cachedConfig *config.Config
	return &Factory{
		Config: func() (*config.Config, error) {
			if cachedConfig != nil {
				return cachedConfig, nil
			}
			cfg, err := config.Load()
			if err != nil {
				return nil, err
			}
			cachedConfig = cfg
			return cfg, nil
		},
	}
}

func (f *Factory) BaseRepo() (Repo, error) {
	cfg, err := f.Config()
	if err != nil {
		return Repo{}, err
	}

	if f.RepoOverride != "" {
		repo, err := RepoFromFullName(f.RepoOverride)
		if err != nil {
			return Repo{}, err
		}
		// If no host on the repo, use default or override
		if repo.Host == "" {
			if f.HostOverride != "" {
				repo.Host = f.HostOverride
			} else {
				_, host, err := cfg.DefaultHost()
				if err != nil {
					return Repo{}, err
				}
				repo.Host = host
			}
		}
		return repo, nil
	}

	return RepoFromGitRemotes(cfg)
}

func (f *Factory) Client(hostname string) (*forgejo.Client, error) {
	cfg, err := f.Config()
	if err != nil {
		return nil, err
	}

	token, err := cfg.TokenForHost(hostname)
	if err != nil {
		return nil, err
	}

	host, err := cfg.HostByName(hostname)
	if err != nil {
		return nil, err
	}

	protocol := "https"
	if host.GitProtocol != "" {
		protocol = host.GitProtocol
	}
	_ = protocol // used for git clone URL construction, not API

	baseURL := fmt.Sprintf("https://%s", hostname)
	return forgejo.NewClient(baseURL, forgejo.SetToken(token))
}

// ClientForRepo creates a client for the repo's host
func (f *Factory) ClientForRepo(repo Repo) (*forgejo.Client, error) {
	return f.Client(repo.Host)
}

// ResolveLabelIDs converts label names to IDs for a repository
func ResolveLabelIDs(client *forgejo.Client, owner, repo string, names []string) ([]int64, error) {
	if len(names) == 0 {
		return nil, nil
	}
	labels, _, err := client.ListRepoLabels(owner, repo, forgejo.ListLabelsOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing labels: %w", err)
	}
	nameToID := make(map[string]int64)
	for _, l := range labels {
		nameToID[strings.ToLower(l.Name)] = l.ID
	}
	var ids []int64
	for _, name := range names {
		id, ok := nameToID[strings.ToLower(name)]
		if !ok {
			return nil, fmt.Errorf("label not found: %s", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ResolveMilestoneID converts a milestone name to its ID
func ResolveMilestoneID(client *forgejo.Client, owner, repo, name string) (int64, error) {
	ms, _, err := client.GetMilestoneByName(owner, repo, name)
	if err != nil {
		return 0, fmt.Errorf("milestone not found: %s", name)
	}
	return ms.ID, nil
}

// TotalCount extracts the X-Total-Count header from a Forgejo API response.
// Returns 0 if the header is not present.
func TotalCount(resp *forgejo.Response) int {
	if resp == nil || resp.Response == nil {
		return 0
	}
	s := resp.Header.Get("X-Total-Count")
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}
