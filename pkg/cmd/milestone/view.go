package milestone

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	Name       string
	Web        bool
	JSONOutput bool
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view <milestone>",
		Short: "View a milestone",
		Long:  "Display a milestone identified by title or ID.",
		Example: `  $ fj milestone view v1.0
  $ fj milestone view v1.0 --web
  $ fj milestone view v1.0 --json`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
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

	ms, _, err := client.GetMilestoneByName(repo.Owner, repo.Name, opts.Name)
	if err != nil {
		return fmt.Errorf("getting milestone: %w", err)
	}

	url := webURL(repo, ms.ID)

	if opts.Web {
		return cmdutil.OpenInBrowser(url)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ms)
	}

	fmt.Fprintf(os.Stdout, "%s\n", ms.Title)
	fmt.Fprintf(os.Stdout, "State: %s\n", ms.State)
	if due := dueDateStr(ms); due != "" {
		fmt.Fprintf(os.Stdout, "Due date: %s\n", due)
	}
	if total := ms.OpenIssues + ms.ClosedIssues; total > 0 {
		fmt.Fprintf(os.Stdout, "Progress: %d/%d issues closed (%d%%)\n",
			ms.ClosedIssues, total, ms.ClosedIssues*100/total)
	} else {
		fmt.Fprintf(os.Stdout, "Progress: no issues\n")
	}
	fmt.Fprintf(os.Stdout, "Created: %s\n", output.RelativeTimeStr(ms.Created))

	if ms.Description != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", output.RenderMarkdown(ms.Description))
	}

	fmt.Fprintf(os.Stdout, "\nView this milestone on the web: %s\n", url)
	return nil
}
