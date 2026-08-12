package comment

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Args       []string
	Limit      int
	JSONOutput bool
}

func NewCmdList(f *cmdutil.Factory, k Kind) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list " + k.Resolver.ArgSpec(k.Arg),
		Short:   "List comments on " + articleA(k.Noun) + " " + k.Noun,
		Aliases: []string{"ls"},
		Example: fmt.Sprintf(`  $ %s list 42
  $ %s list 42 --limit 100
  $ %s list 42 --json`, k.CLI, k.CLI, k.CLI),
		Args: k.Resolver.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return listRun(opts, k)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of comments to list")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func listRun(opts *listOptions, k Kind) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, err := k.Resolver.Number(opts.Factory, repo, opts.Args)
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	pageSize := min(opts.Limit, 50)
	listOpt := forgejo.ListIssueCommentOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: pageSize},
	}

	var all []*forgejo.Comment
	page := 1
	for len(all) < opts.Limit {
		listOpt.Page = page
		comments, _, err := client.ListIssueComments(repo.Owner, repo.Name, index, listOpt)
		if err != nil {
			return fmt.Errorf("listing comments: %w", err)
		}
		if len(comments) == 0 {
			break
		}
		all = append(all, comments...)
		if len(comments) < pageSize {
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
		fmt.Fprintf(os.Stderr, "No comments on %s #%d in %s\n", k.Noun, index, repo.FullName())
		return nil
	}

	fmt.Fprintf(os.Stdout, "\nShowing %d comments on %s #%d in %s\n\n",
		len(all), k.Noun, index, repo.FullName())

	t := output.NewTable("ID", "AUTHOR", "UPDATED", "BODY").Flexible(3)
	for _, c := range all {
		author := ""
		if c.Poster != nil {
			author = c.Poster.UserName
		}
		t.AddRow(
			output.Colorize(output.Green, strconv.FormatInt(c.ID, 10)),
			author,
			output.Colorize(output.Gray, output.RelativeTimeStr(c.Updated)),
			output.Sanitize(c.Body),
		)
	}
	t.Render(os.Stdout)
	return nil
}
