package release

import (
	"encoding/json"
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	Tag        string
	Web        bool
	JSONOutput bool
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view [<tag>]",
		Short: "View a release",
		Long:  "View a release by tag. If no tag is given, shows the latest release.",
		Example: `  $ fj release view
  $ fj release view v1.2.0
  $ fj release view v1.2.0 --web
  $ fj release view v1.2.0 --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Tag = args[0]
			}
			return viewRun(opts)
		},
	}

	cmdutil.AddWebFlag(cmd, &opts.Web)
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func viewRun(opts *viewOptions) error {
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

	if opts.Web {
		return cmdutil.OpenInBrowser(rel.HTMLURL)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rel)
	}

	title := rel.Title
	if title == "" {
		title = rel.TagName
	}
	fmt.Fprintf(os.Stdout, "%s (%s)\n", title, rel.TagName)

	typ := "release"
	switch {
	case rel.IsDraft:
		typ = "draft"
	case rel.IsPrerelease:
		typ = "pre-release"
	}
	fmt.Fprintf(os.Stdout, "Type: %s\n", typ)
	if rel.Target != "" {
		fmt.Fprintf(os.Stdout, "Target: %s\n", rel.Target)
	}
	if rel.Publisher != nil {
		fmt.Fprintf(os.Stdout, "Author: %s\n", rel.Publisher.UserName)
	}
	fmt.Fprintf(os.Stdout, "Published: %s\n", output.RelativeTimeStr(rel.PublishedAt))

	if len(rel.Attachments) > 0 {
		fmt.Fprintf(os.Stdout, "\nAssets (%d):\n", len(rel.Attachments))
		for _, a := range rel.Attachments {
			fmt.Fprintf(os.Stdout, "  %s  %s  (%d downloads)\n",
				output.Colorize(output.Cyan, a.Name),
				humanSize(a.Size),
				a.DownloadCount,
			)
		}
	}

	if rel.Note != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", rel.Note)
	}

	fmt.Fprintf(os.Stdout, "\nView this release on the web: %s\n", rel.HTMLURL)
	return nil
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
