package repo

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/git"
	"github.com/riadafridishibly/fj/internal/output"
)

type forkOptions struct {
	Factory    *cmdutil.Factory
	Repo       string
	Org        string
	ForkName   string
	Clone      bool
	JSONOutput bool
}

func NewCmdFork(f *cmdutil.Factory) *cobra.Command {
	opts := &forkOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "fork [<owner/repo>]",
		Short: "Create a fork of a repository",
		Example: `  $ fj repo fork owner/repo
  $ fj repo fork owner/repo --org myorg
  $ fj repo fork owner/repo --clone
  $ fj repo fork`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Repo = args[0]
			}
			return forkRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Org, "org", "o", "", "Fork into an organization")
	cmd.Flags().StringVar(&opts.ForkName, "fork-name", "", "Name for the forked repository")
	cmd.Flags().BoolVar(&opts.Clone, "clone", false, "Clone the fork after creating it")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func forkRun(opts *forkOptions) error {
	var repo cmdutil.Repo
	var err error

	if opts.Repo != "" {
		repo, err = cmdutil.RepoFromFullName(opts.Repo)
		if err != nil {
			return err
		}
		cfg, err := opts.Factory.Config()
		if err != nil {
			return err
		}
		_, host, err := cfg.DefaultHost()
		if err != nil {
			return err
		}
		repo.Host = host
	} else {
		repo, err = opts.Factory.BaseRepo()
		if err != nil {
			return err
		}
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	forkOpt := forgejo.CreateForkOption{}
	if opts.Org != "" {
		forkOpt.Organization = &opts.Org
	}
	if opts.ForkName != "" {
		forkOpt.Name = &opts.ForkName
	}

	forked, _, err := client.CreateFork(repo.Owner, repo.Name, forkOpt)
	if err != nil {
		return fmt.Errorf("forking repository: %w", err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, forked)
	}

	fmt.Fprintf(os.Stdout, "%s\n", forked.HTMLURL)

	if opts.Clone {
		cfg, err := opts.Factory.Config()
		if err != nil {
			return err
		}
		host, err := cfg.HostByName(repo.Host)
		if err != nil {
			return err
		}
		cloneURL := forked.CloneURL
		if host.GitProtocol == "ssh" {
			cloneURL = forked.SSHURL
		}
		if err := git.Clone(cloneURL, ""); err != nil {
			return fmt.Errorf("cloning fork: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Cloned fork to ./%s\n", forked.Name)
	}

	return nil
}
