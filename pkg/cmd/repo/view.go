package repo

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	Repo       string
	Web        bool
	JSONOutput cmdutil.JSONFlags
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view [<owner/repo>]",
		Short: "View a repository",
		Example: `  $ fj repo view
  $ fj repo view owner/repo
  $ fj repo view --web
  $ fj repo view --json nameWithOwner,defaultBranchRef
  $ fj repo view --json owner --jq .owner.login`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Repo = args[0]
			}
			return viewRun(opts)
		},
	}

	cmdutil.AddWebFlag(cmd, &opts.Web)
	cmdutil.AddJSONFlags(cmd, &opts.JSONOutput, repoFields, nil, true)

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
		if repo.Host, err = opts.Factory.Host(); err != nil {
			return err
		}
	} else {
		repo, err = opts.Factory.BaseRepo()
		if err != nil {
			return err
		}
	}

	r, err := getRepo(opts.Factory, repo)
	if err != nil {
		return err
	}

	if opts.Web {
		return cmdutil.OpenInBrowser(r.HTMLURL)
	}

	if opts.JSONOutput.Enabled() {
		data, err := repoJSON(opts.Factory, repo.Host, r, &opts.JSONOutput)
		if err != nil {
			return err
		}
		return opts.JSONOutput.Write(os.Stdout, data)
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
