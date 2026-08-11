package pr

import (
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
	Factory         *cmdutil.Factory
	Number          string
	Comments        bool
	ShowTimeline    bool
	TimelineInclude []string
	TimelineExclude []string
	Web             bool
	JSONOutput      bool
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view <number>",
		Short: "View a pull request",
		Example: `  $ fj pr view 42
  $ fj pr view 42 --comments
  $ fj pr view 42 --show-timeline=false
  $ fj pr view 42 --timeline-exclude commits
  $ fj pr view 42 --timeline-include refs --timeline-exclude commits
  $ fj pr view 42 --web
  $ fj pr view 42 --json`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			return viewRun(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.Comments, "comments", "c", false, "View pull request comments")
	cmd.Flags().BoolVar(&opts.ShowTimeline, "show-timeline", true, "Show pull request events, such as commit references and label changes")
	cmdutil.AddTimelineFilterFlags(cmd, &opts.TimelineInclude, &opts.TimelineExclude, cmdutil.SubjectPull)
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
		return cmdutil.FlagErrorf("invalid pull request number: %s", opts.Number)
	}

	timeline, err := cmdutil.NewTimelineFilter(opts.TimelineInclude, opts.TimelineExclude)
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	pr, _, err := client.GetPullRequest(repo.Owner, repo.Name, index)
	if err != nil {
		return fmt.Errorf("getting pull request: %w", err)
	}

	if opts.Web {
		return cmdutil.OpenInBrowser(pr.HTMLURL)
	}

	if opts.JSONOutput {
		result := map[string]any{
			"pull_request": pr,
		}
		if opts.Comments {
			comments, _, err := client.ListIssueComments(repo.Owner, repo.Name, index, forgejo.ListIssueCommentOptions{})
			if err != nil {
				return fmt.Errorf("listing comments: %w", err)
			}
			result["comments"] = comments
		}
		if opts.ShowTimeline {
			events, err := opts.Factory.Timeline(repo, index)
			if err != nil {
				return err
			}
			result["timeline"] = timeline.Apply(events)
		}
		return output.PrintJSON(os.Stdout, result)
	}

	// Text output
	fmt.Fprintf(os.Stdout, "%s #%d\n", pr.Title, pr.Index)

	status := string(pr.State)
	if pr.HasMerged {
		status = "merged"
	}
	fmt.Fprintf(os.Stdout, "State: %s\n", status)

	if pr.Poster != nil {
		fmt.Fprintf(os.Stdout, "Author: %s\n", pr.Poster.UserName)
	}

	if pr.Head != nil && pr.Base != nil {
		fmt.Fprintf(os.Stdout, "Branch: %s → %s\n", pr.Head.Ref, pr.Base.Ref)
	}

	if len(pr.Labels) > 0 {
		var names []string
		for _, l := range pr.Labels {
			names = append(names, l.Name)
		}
		fmt.Fprintf(os.Stdout, "Labels: %s\n", strings.Join(names, ", "))
	}

	if len(pr.Assignees) > 0 {
		var names []string
		for _, a := range pr.Assignees {
			names = append(names, a.UserName)
		}
		fmt.Fprintf(os.Stdout, "Assignees: %s\n", strings.Join(names, ", "))
	}

	if pr.Milestone != nil {
		fmt.Fprintf(os.Stdout, "Milestone: %s\n", pr.Milestone.Title)
	}

	fmt.Fprintf(os.Stdout, "Mergeable: %v\n", pr.Mergeable)

	if pr.Created != nil {
		fmt.Fprintf(os.Stdout, "Created: %s\n", output.RelativeTimeStr(*pr.Created))
	}

	if pr.Body != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", output.RenderMarkdown(pr.Body))
	}

	fmt.Fprintf(os.Stdout, "\nView this pull request on the web: %s\n", pr.HTMLURL)

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
				fmt.Fprintf(
					os.Stdout, "\n%s commented %s:\n%s\n",
					author,
					output.RelativeTimeStr(c.Created),
					output.RenderMarkdown(c.Body),
				)
			}
		}
	}

	if opts.ShowTimeline {
		events, err := opts.Factory.Timeline(repo, index)
		if err != nil {
			return err
		}
		if lines := cmdutil.TimelineLines(timeline.Apply(events), cmdutil.SubjectPull, repo); len(lines) > 0 {
			fmt.Fprintf(os.Stdout, "\n--- Timeline (%d) ---\n", len(lines))
			for _, l := range lines {
				fmt.Fprintf(os.Stdout, "%s\n", l)
			}
		}
	}

	return nil
}
