package repo

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	Repo       string
	Web        bool
	JSONOutput bool
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view [<owner/repo>]",
		Short: "View a repository",
		Example: `  $ fj repo view
  $ fj repo view owner/repo
  $ fj repo view --web
  $ fj repo view --json`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Repo = args[0]
			}
			return viewRun(opts)
		},
	}

	cmdutil.AddWebFlag(cmd, &opts.Web)
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func viewRun(opts *viewOptions) error {
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

	r, _, err := client.GetRepo(repo.Owner, repo.Name)
	if err != nil {
		return fmt.Errorf("getting repository: %w", err)
	}

	if opts.Web {
		return cmdutil.OpenInBrowser(r.HTMLURL)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}

	fmt.Fprintf(os.Stdout, "%s\n", r.FullName)
	if r.Description != "" {
		fmt.Fprintf(os.Stdout, "%s\n", r.Description)
	}
	fmt.Fprintln(os.Stdout)

	if r.Website != "" {
		fmt.Fprintf(os.Stdout, "Homepage:  %s\n", r.Website)
	}
	visibility := "public"
	if r.Private {
		visibility = "private"
	}
	fmt.Fprintf(os.Stdout, "Visibility: %s\n", visibility)
	fmt.Fprintf(os.Stdout, "Default branch: %s\n", r.DefaultBranch)

	fmt.Fprintf(os.Stdout, "Stars: %d  Forks: %d  Open issues: %d\n", r.Stars, r.Forks, r.OpenIssues)
	fmt.Fprintf(os.Stdout, "\nClone URL: %s\n", r.CloneURL)
	if r.SSHURL != "" {
		fmt.Fprintf(os.Stdout, "SSH URL:   %s\n", r.SSHURL)
	}
	fmt.Fprintf(os.Stdout, "Web URL:   %s\n", r.HTMLURL)

	return nil
}
