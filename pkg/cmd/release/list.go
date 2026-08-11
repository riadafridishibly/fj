package release

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory           *cmdutil.Factory
	Limit             int
	ExcludeDrafts     bool
	ExcludePreRelease bool
	JSONOutput        bool
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List releases in a repository",
		Aliases: []string{"ls"},
		Example: `  $ fj release list
  $ fj release list --limit 50
  $ fj release list --exclude-drafts --exclude-pre-releases
  $ fj release list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of releases to list")
	cmd.Flags().BoolVar(&opts.ExcludeDrafts, "exclude-drafts", false, "Exclude draft releases")
	cmd.Flags().BoolVar(&opts.ExcludePreRelease, "exclude-pre-releases", false, "Exclude pre-releases")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func listRun(opts *listOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	pageSize := min(opts.Limit, 50)

	listOpt := forgejo.ListReleasesOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: pageSize},
	}
	if opts.ExcludeDrafts {
		f := false
		listOpt.IsDraft = &f
	}
	if opts.ExcludePreRelease {
		f := false
		listOpt.IsPreRelease = &f
	}

	var all []*forgejo.Release
	page := 1
	for len(all) < opts.Limit {
		listOpt.Page = page
		rels, _, err := client.ListReleases(repo.Owner, repo.Name, listOpt)
		if err != nil {
			return fmt.Errorf("listing releases: %w", err)
		}
		if len(rels) == 0 {
			break
		}
		all = append(all, rels...)
		if len(rels) < pageSize {
			break
		}
		page++
	}
	if len(all) > opts.Limit {
		all = all[:opts.Limit]
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, all)
	}

	if len(all) == 0 {
		fmt.Fprintf(os.Stderr, "No releases found in %s\n", repo.FullName())
		return nil
	}

	fmt.Fprintf(os.Stdout, "\nShowing %d releases in %s\n\n", len(all), repo.FullName())

	t := output.NewTable("TAG", "TITLE", "TYPE", "PUBLISHED").Flexible(1)
	for _, r := range all {
		typ := "release"
		typeColor := output.Green
		switch {
		case r.IsDraft:
			typ = "draft"
			typeColor = output.Yellow
		case r.IsPrerelease:
			typ = "pre-release"
			typeColor = output.Magenta
		}
		t.AddRow(
			output.Colorize(output.Cyan, r.TagName),
			output.Sanitize(r.Title),
			output.Colorize(typeColor, typ),
			output.Colorize(output.Gray, output.RelativeTimeStr(r.PublishedAt)),
		)
	}
	t.Render(os.Stdout)
	return nil
}
