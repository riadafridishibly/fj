package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Limit      int
	Owner      string
	Visibility string
	Fork       bool
	Source     bool
	JSONOutput bool
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list [<owner>]",
		Short:   "List repositories owned by a user or organization",
		Aliases: []string{"ls"},
		Example: `  $ fj repo list
  $ fj repo list --limit 50
  $ fj repo list myorg
  $ fj repo list --json`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Owner = args[0]
			}
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of repositories to list")
	cmd.Flags().StringVar(&opts.Visibility, "visibility", "", "Filter by visibility: public, private")
	cmd.Flags().BoolVar(&opts.Fork, "fork", false, "Show only forks")
	cmd.Flags().BoolVar(&opts.Source, "source", false, "Show only non-forks")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func listRun(opts *listOptions) error {
	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}

	host, hostname, err := cfg.DefaultHost()
	if err != nil {
		return err
	}

	client, err := opts.Factory.Client(hostname)
	if err != nil {
		return err
	}

	pageSize := min(opts.Limit, 50)

	var allRepos []*forgejo.Repository
	var totalCount int
	page := 1
	for len(allRepos) < opts.Limit {
		var repos []*forgejo.Repository
		var resp *forgejo.Response
		var err error

		if opts.Owner != "" {
			repos, resp, err = client.ListUserRepos(opts.Owner, forgejo.ListReposOptions{
				ListOptions: forgejo.ListOptions{Page: page, PageSize: pageSize},
			})
		} else {
			repos, resp, err = client.ListMyRepos(forgejo.ListReposOptions{
				ListOptions: forgejo.ListOptions{Page: page, PageSize: pageSize},
			})
		}
		if err != nil {
			return fmt.Errorf("listing repositories: %w", err)
		}
		if page == 1 {
			totalCount = cmdutil.TotalCount(resp)
		}
		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			if opts.Visibility == "public" && r.Private {
				continue
			}
			if opts.Visibility == "private" && !r.Private {
				continue
			}
			if opts.Fork && !r.Fork {
				continue
			}
			if opts.Source && r.Fork {
				continue
			}
			allRepos = append(allRepos, r)
			if len(allRepos) >= opts.Limit {
				break
			}
		}
		page++
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allRepos)
	}

	if len(allRepos) == 0 {
		fmt.Fprintln(os.Stderr, "No repositories found")
		return nil
	}

	// Status line
	owner := opts.Owner
	if owner == "" {
		owner = host.User
	}
	if totalCount > 0 {
		fmt.Fprintf(os.Stdout, "\nShowing %d of %d repositories in @%s\n\n",
			len(allRepos), totalCount, owner)
	} else {
		fmt.Fprintf(os.Stdout, "\nShowing %d repositories in @%s\n\n",
			len(allRepos), owner)
	}

	// Table with headers
	t := output.NewTable("NAME", "DESCRIPTION", "INFO", "UPDATED")
	for _, r := range allRepos {
		var info []string
		if r.Private {
			info = append(info, output.Colorize(output.Yellow, "private"))
		} else {
			info = append(info, output.Colorize(output.Green, "public"))
		}
		if r.Fork {
			info = append(info, output.Colorize(output.Cyan, "fork"))
		}
		if r.Archived {
			info = append(info, output.Colorize(output.Red, "archived"))
		}

		t.AddRow(
			output.Colorize(output.Bold, r.FullName),
			output.Truncate(r.Description, 60),
			strings.Join(info, ", "),
			output.Colorize(output.Gray, output.RelativeTimeStr(r.Updated)),
		)
	}
	t.Render(os.Stdout)
	return nil
}
