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

type createOptions struct {
	Factory       *cmdutil.Factory
	Name          string
	Description   string
	Private       bool
	Clone         bool
	GitIgnore     string
	License       string
	DefaultBranch string
	Org           string
	JSONOutput    bool
}

func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "create [<name>]",
		Short: "Create a new repository",
		Example: `  $ fj repo create my-project
  $ fj repo create my-project --private --description "My project"
  $ fj repo create my-project --org myorg
  $ fj repo create my-project --clone
  $ fj repo create my-project --json`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Name = args[0]
			}
			if opts.Name == "" {
				return cmdutil.FlagErrorf("repository name is required")
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "Description of the repository")
	cmd.Flags().BoolVar(&opts.Private, "private", false, "Make the repository private")
	cmd.Flags().BoolVarP(&opts.Clone, "clone", "c", false, "Clone the repository after creating it")
	cmd.Flags().StringVar(&opts.GitIgnore, "gitignore", "", "Gitignore template to use")
	cmd.Flags().StringVarP(&opts.License, "license", "l", "", "License to use")
	cmd.Flags().StringVar(&opts.DefaultBranch, "default-branch", "", "Default branch name")
	cmd.Flags().StringVarP(&opts.Org, "org", "o", "", "Create in an organization")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func createRun(opts *createOptions) error {
	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}

	_, hostname, err := cfg.DefaultHost()
	if err != nil {
		return err
	}

	client, err := opts.Factory.Client(hostname)
	if err != nil {
		return err
	}

	createOpt := forgejo.CreateRepoOption{
		Name:          opts.Name,
		Description:   opts.Description,
		Private:       opts.Private,
		AutoInit:      true,
		Gitignores:    opts.GitIgnore,
		License:       opts.License,
		DefaultBranch: opts.DefaultBranch,
	}

	var repo *forgejo.Repository
	if opts.Org != "" {
		repo, _, err = client.CreateOrgRepo(opts.Org, createOpt)
	} else {
		repo, _, err = client.CreateRepo(createOpt)
	}
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, repo)
	}

	fmt.Fprintf(os.Stdout, "%s\n", repo.HTMLURL)

	if opts.Clone {
		host, err := cfg.HostByName(hostname)
		if err != nil {
			return err
		}
		cloneURL := repo.CloneURL
		if host.GitProtocol == "ssh" {
			cloneURL = repo.SSHURL
		}
		if err := git.Clone(cloneURL, ""); err != nil {
			return fmt.Errorf("cloning repository: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Cloned to ./%s\n", repo.Name)
	}

	return nil
}
