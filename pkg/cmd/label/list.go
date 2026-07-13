package label

import (
	"encoding/json"
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Limit      int
	JSONOutput bool
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List labels in a repository",
		Aliases: []string{"ls"},
		Example: `  $ fj label list
  $ fj label list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of labels to list")
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

	var allLabels []*forgejo.Label
	page := 1
	for len(allLabels) < opts.Limit {
		labels, _, err := client.ListRepoLabels(repo.Owner, repo.Name, forgejo.ListLabelsOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: pageSize},
		})
		if err != nil {
			return fmt.Errorf("listing labels: %w", err)
		}
		if len(labels) == 0 {
			break
		}
		allLabels = append(allLabels, labels...)
		if len(labels) < pageSize {
			break
		}
		page++
	}
	if len(allLabels) > opts.Limit {
		allLabels = allLabels[:opts.Limit]
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allLabels)
	}

	if len(allLabels) == 0 {
		fmt.Fprintf(os.Stderr, "No labels found in %s\n", repo.FullName())
		return nil
	}

	fmt.Fprintf(os.Stdout, "\nShowing %d labels in %s\n\n", len(allLabels), repo.FullName())

	t := output.NewTable("NAME", "COLOR", "DESCRIPTION").Flexible(2)
	for _, l := range allLabels {
		t.AddRow(
			output.Colorize(output.Cyan, l.Name),
			"#"+l.Color,
			output.Sanitize(l.Description),
		)
	}
	t.Render(os.Stdout)
	return nil
}
