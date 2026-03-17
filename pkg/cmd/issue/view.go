package issue

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	Comments   bool
	Web        bool
	JSONOutput bool
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view <number>",
		Short: "View an issue",
		Example: `  $ fj issue view 42
  $ fj issue view 42 --comments
  $ fj issue view 42 --web
  $ fj issue view 42 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			return viewRun(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.Comments, "comments", "c", false, "View issue comments")
	cmdutil.AddWebFlag(cmd, &opts.Web)
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func viewRun(opts *viewOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid issue number: %s", opts.Number)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	issue, _, err := client.GetIssue(repo.Owner, repo.Name, index)
	if err != nil {
		return fmt.Errorf("getting issue: %w", err)
	}

	if opts.Web {
		return cmdutil.OpenInBrowser(issue.HTMLURL)
	}

	if opts.JSONOutput {
		result := map[string]any{
			"issue": issue,
		}
		if opts.Comments {
			comments, _, err := client.ListIssueComments(repo.Owner, repo.Name, index, forgejo.ListIssueCommentOptions{})
			if err != nil {
				return fmt.Errorf("listing comments: %w", err)
			}
			result["comments"] = comments
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Text output
	fmt.Fprintf(os.Stdout, "%s #%d\n", issue.Title, issue.Index)
	fmt.Fprintf(os.Stdout, "State: %s\n", issue.State)
	if issue.Poster != nil {
		fmt.Fprintf(os.Stdout, "Author: %s\n", issue.Poster.UserName)
	}

	if len(issue.Labels) > 0 {
		var names []string
		for _, l := range issue.Labels {
			names = append(names, l.Name)
		}
		fmt.Fprintf(os.Stdout, "Labels: %s\n", strings.Join(names, ", "))
	}

	if len(issue.Assignees) > 0 {
		var names []string
		for _, a := range issue.Assignees {
			names = append(names, a.UserName)
		}
		fmt.Fprintf(os.Stdout, "Assignees: %s\n", strings.Join(names, ", "))
	}

	if issue.Milestone != nil {
		fmt.Fprintf(os.Stdout, "Milestone: %s\n", issue.Milestone.Title)
	}

	fmt.Fprintf(os.Stdout, "Created: %s\n", output.RelativeTimeStr(issue.Created))

	if issue.Body != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", issue.Body)
	}

	fmt.Fprintf(os.Stdout, "\nView this issue on the web: %s\n", issue.HTMLURL)

	if opts.Comments {
		comments, _, err := client.ListIssueComments(repo.Owner, repo.Name, index, forgejo.ListIssueCommentOptions{})
		if err != nil {
			return fmt.Errorf("listing comments: %w", err)
		}
		if len(comments) > 0 {
			fmt.Fprintf(os.Stdout, "\n--- Comments (%d) ---\n", len(comments))
			for _, c := range comments {
				author := "unknown"
				if c.Poster != nil {
					author = c.Poster.UserName
				}
				fmt.Fprintf(os.Stdout, "\n%s commented %s:\n%s\n",
					author,
					output.RelativeTimeStr(c.Created),
					c.Body,
				)
			}
		}
	}

	return nil
}
