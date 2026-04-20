package release

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type downloadOptions struct {
	Factory  *cmdutil.Factory
	Tag      string
	Dir      string
	Patterns []string
}

func NewCmdDownload(f *cmdutil.Factory) *cobra.Command {
	opts := &downloadOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "download [<tag>]",
		Short: "Download release assets",
		Long:  "Download assets from a release. If no tag is given, downloads from the latest release.",
		Example: `  $ fj release download v1.2.0
  $ fj release download v1.2.0 --dir ./artifacts
  $ fj release download v1.2.0 --pattern '*linux*' --pattern '*.tar.gz'`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Tag = args[0]
			}
			return downloadRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Dir, "dir", "D", ".", "Directory to download assets into")
	cmd.Flags().StringSliceVarP(&opts.Patterns, "pattern", "p", nil, "Download only assets matching glob pattern(s)")

	return cmd
}

func downloadRun(opts *downloadOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	var rel *forgejo.Release
	if opts.Tag == "" {
		rel, _, err = client.GetLatestRelease(repo.Owner, repo.Name)
	} else {
		rel, _, err = client.GetReleaseByTag(repo.Owner, repo.Name, opts.Tag)
	}
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	if len(rel.Attachments) == 0 {
		fmt.Fprintf(os.Stderr, "No assets in release %s\n", rel.TagName)
		return nil
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}
	token, err := cfg.TokenForHost(repo.Host)
	if err != nil {
		return err
	}

	count := 0
	for _, a := range rel.Attachments {
		if !matchesAny(a.Name, opts.Patterns) {
			continue
		}
		dest := filepath.Join(opts.Dir, a.Name)
		if err := downloadFile(a.DownloadURL, token, dest); err != nil {
			return fmt.Errorf("downloading %s: %w", a.Name, err)
		}
		fmt.Fprintf(os.Stderr, "✓ Downloaded %s\n", a.Name)
		count++
	}

	if count == 0 {
		fmt.Fprintf(os.Stderr, "No assets matched the given patterns\n")
	}
	return nil
}

func matchesAny(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
		if strings.Contains(name, p) {
			return true
		}
	}
	return false
}

func downloadFile(url, token, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
