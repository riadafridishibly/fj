package repo

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/git"
)

type cloneOptions struct {
	Factory   *cmdutil.Factory
	Repo      string
	Directory string
}

func NewCmdClone(f *cmdutil.Factory) *cobra.Command {
	opts := &cloneOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "clone <owner/repo> [<directory>]",
		Short: "Clone a repository locally",
		Example: `  $ fj repo clone owner/repo
  $ fj repo clone owner/repo my-directory`,
		Args: cmdutil.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Repo = args[0]
			if len(args) > 1 {
				opts.Directory = args[1]
			}
			return cloneRun(opts)
		},
	}

	return cmd
}

func cloneRun(opts *cloneOptions) error {
	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}

	_, hostname, err := cfg.DefaultHost()
	if err != nil {
		return err
	}

	host, err := cfg.HostByName(hostname)
	if err != nil {
		return err
	}

	// If it's already a full URL, clone directly
	if strings.HasPrefix(opts.Repo, "http://") || strings.HasPrefix(opts.Repo, "https://") || strings.HasPrefix(opts.Repo, "git@") {
		if err := git.Clone(opts.Repo, opts.Directory); err != nil {
			return fmt.Errorf("cloning: %w", err)
		}
		return nil
	}

	repo, err := cmdutil.RepoFromFullName(opts.Repo)
	if err != nil {
		return err
	}

	client, err := opts.Factory.Client(hostname)
	if err != nil {
		return err
	}

	r, _, err := client.GetRepo(repo.Owner, repo.Name)
	if err != nil {
		return fmt.Errorf("getting repository: %w", err)
	}

	cloneURL := r.CloneURL
	if host.GitProtocol == "ssh" {
		cloneURL = r.SSHURL
	}

	dir := opts.Directory
	if dir == "" {
		dir = r.Name
	}

	if err := git.Clone(cloneURL, dir); err != nil {
		return fmt.Errorf("cloning: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Cloned %s to ./%s\n", r.FullName, dir)
	return nil
}
