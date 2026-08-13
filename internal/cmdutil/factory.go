package cmdutil

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/config"
	"github.com/riadafridishibly/fj/internal/debug"
)

type Factory struct {
	Config func() (*config.Config, error)

	// RepoOverride is set by the -R flag
	RepoOverride string

	// clients memoizes API clients by hostname, so an invocation that first
	// resolves a pull request and then acts on it shares one client.
	clients map[string]*forgejo.Client
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
		return f.RepoFromArg(f.RepoOverride)
	}

	return RepoFromGitRemotes(cfg)
}

// RepoFromArg resolves an explicit [HOST/]OWNER/REPO selector, filling in
// the configured default host when the selector omits it.
func (f *Factory) RepoFromArg(name string) (Repo, error) {
	repo, err := RepoFromFullName(name)
	if err != nil {
		return Repo{}, err
	}
	if repo.Host != "" {
		return repo, nil
	}
	cfg, err := f.Config()
	if err != nil {
		return Repo{}, err
	}
	_, host, err := cfg.DefaultHost()
	if err != nil {
		return Repo{}, err
	}
	repo.Host = host
	return repo, nil
}

func (f *Factory) Client(hostname string) (*forgejo.Client, error) {
	if client, ok := f.clients[hostname]; ok {
		return client, nil
	}

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

	scheme := "https"
	if os.Getenv("FJ_INSECURE") != "" {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, hostname)
	client, err := NewForgejoClient(baseURL, token)
	if err != nil {
		return nil, err
	}
	if f.clients == nil {
		f.clients = make(map[string]*forgejo.Client)
	}
	f.clients[hostname] = client
	return client, nil
}

// NewForgejoClient builds a forgejo.Client with fj's standard options
// (auth token + DEBUG-aware HTTP transport). Shared by the factory and
// by auth commands that construct clients outside the factory (login,
// status) to validate credentials before they land in config.
func NewForgejoClient(baseURL, token string) (*forgejo.Client, error) {
	return forgejo.NewClient(
		baseURL,
		forgejo.SetToken(token),
		forgejo.SetHTTPClient(debug.WrapClient(nil)),
	)
}

// APIClient builds an api.Client for endpoints the Forgejo SDK does not
// cover. It authenticates against the repo's host with the same token and
// debug-aware transport as the SDK client.
func (f *Factory) APIClient(repo Repo) (*api.Client, error) {
	cfg, err := f.Config()
	if err != nil {
		return nil, err
	}
	token, err := cfg.TokenForHost(repo.Host)
	if err != nil {
		return nil, err
	}
	scheme := "https"
	if os.Getenv("FJ_INSECURE") != "" {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, repo.Host)
	return api.NewClient(baseURL, token, debug.WrapClient(nil)), nil
}

// APIGet performs an authenticated GET against /api/v1<path> on the repo's
// host and returns the response. Used for endpoints the Forgejo SDK does
// not cover. Caller owns closing resp.Body.
func (f *Factory) APIGet(repo Repo, path string) (*http.Response, error) {
	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	return client.Get(path)
}

// ClientForRepo creates a client for the repo's host
func (f *Factory) ClientForRepo(repo Repo) (*forgejo.Client, error) {
	return f.Client(repo.Host)
}

// ResolveLabelIDs converts label names to IDs for a repository.
// If a label does not exist, it is automatically created.
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
			label, _, err := client.CreateLabel(owner, repo, forgejo.CreateLabelOption{
				Name:  name,
				Color: RandomLabelColor(),
			})
			if err != nil {
				return nil, fmt.Errorf("creating label %q: %w", name, err)
			}
			id = label.ID
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// RandomLabelColor returns a random hex color suitable for labels.
func RandomLabelColor() string {
	colors := []string{
		"#0075ca", "#008672", "#a2eeef", "#d876e3",
		"#e4e669", "#ee0701", "#fbca04", "#0e8a16",
		"#c5def5", "#bfdadc", "#f9d0c4", "#d4c5f9",
		"#bfd4f2", "#c2e0c6",
	}
	return colors[rand.IntN(len(colors))]
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
