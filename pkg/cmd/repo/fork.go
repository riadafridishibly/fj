package repo

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/git"
)

type forkOptions struct {
	Factory    *cmdutil.Factory
	Repo       string
	Org        string
	ForkName   string
	Clone      bool
	JSONOutput cmdutil.JSONFlags
}

func NewCmdFork(f *cmdutil.Factory) *cobra.Command {
	opts := &forkOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "fork [<owner/repo>]",
		Short: "Create a fork of a repository",
		Example: `  $ fj repo fork owner/repo
  $ fj repo fork owner/repo --org myorg
  $ fj repo fork owner/repo --clone
  $ fj repo fork
  $ fj repo fork owner/repo --json nameWithOwner,parent`,
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
	cmdutil.AddJSONFlags(cmd, &opts.JSONOutput, repoFields, nil, false)

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
		if repo.Host, err = opts.Factory.Host(); err != nil {
			return err
		}
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

	if opts.JSONOutput.Enabled() {
		return writeRepo(opts.Factory, repo.Host, forked.FullName, &opts.JSONOutput)
	}

	fmt.Fprintf(os.Stdout, "%s\n", forked.HTMLURL)

	if opts.Clone {
		cfg, err := opts.Factory.Config()
		if err != nil {
			return err
		}
		cloneURL := forked.CloneURL
		if cfg.GitProtocol(repo.Host) == "ssh" {
			cloneURL = forked.SSHURL
		}
		if err := git.Clone(cloneURL, ""); err != nil {
			return fmt.Errorf("cloning fork: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Cloned fork to ./%s\n", forked.Name)
	}

	return nil
}
