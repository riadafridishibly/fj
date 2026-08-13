package issue

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

// statusLimit caps how many issues a section lists; whatever the server
// counted beyond that becomes the "And N more" tail.
const statusLimit = 10

type statusOptions struct {
	Factory    *cmdutil.Factory
	JSONOutput bool
}

func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	opts := &statusOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of relevant issues",
		Long: `Show the open issues in this repository that concern you: the ones assigned to
you, the ones mentioning you, and the ones you opened.`,
		Example: `  $ fj issue status
  $ fj issue status --json`,
		Args: cmdutil.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(opts)
		},
	}

	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

// issueSection is one "relevant to you" bucket: the issues to list plus the
// server's unfiltered count, which is what the "And N more" tail reports.
type issueSection struct {
	Issues []*forgejo.Issue `json:"issues"`
	Total  int              `json:"total"`
}

type issueStatus struct {
	Repository string       `json:"repository"`
	Assigned   issueSection `json:"assigned"`
	Mentioned  issueSection `json:"mentioned"`
	Authored   issueSection `json:"authored"`
}

func statusRun(opts *statusOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}
	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}
	me, _, err := client.GetMyUserInfo()
	if err != nil {
		return fmt.Errorf("getting the authenticated user: %w", err)
	}

	// Every section is the same query under a different relationship, so the
	// three differ only in which "by user" filter they set.
	section := func(opt forgejo.ListIssueOption) (issueSection, error) {
		opt.ListOptions = forgejo.ListOptions{Page: 1, PageSize: statusLimit}
		opt.State = forgejo.StateOpen
		opt.Type = forgejo.IssueTypeIssue
		issues, resp, err := client.ListRepoIssues(repo.Owner, repo.Name, opt)
		if err != nil {
			return issueSection{}, fmt.Errorf("listing issues: %w", err)
		}
		return issueSection{Issues: issues, Total: max(cmdutil.TotalCount(resp), len(issues))}, nil
	}

	status := &issueStatus{Repository: repo.FullName()}
	if status.Assigned, err = section(forgejo.ListIssueOption{AssignedBy: me.UserName}); err != nil {
		return err
	}
	if status.Mentioned, err = section(forgejo.ListIssueOption{MentionedBy: me.UserName}); err != nil {
		return err
	}
	if status.Authored, err = section(forgejo.ListIssueOption{CreatedBy: me.UserName}); err != nil {
		return err
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, status)
	}

	w := os.Stdout
	fmt.Fprintf(w, "\n%s\n\n", output.Colorize(output.Bold, "Relevant issues in "+status.Repository))
	output.StatusSection(w, "Issues assigned to you", "There are no issues assigned to you", issueLines(status.Assigned))
	output.StatusSection(w, "Issues mentioning you", "There are no issues mentioning you", issueLines(status.Mentioned))
	output.StatusSection(w, "Issues opened by you", "There are no issues opened by you", issueLines(status.Authored))
	return nil
}

func issueLines(s issueSection) []string {
	lines := make([]string, 0, len(s.Issues)+1)
	for _, issue := range s.Issues {
		lines = append(lines, fmt.Sprintf("%s  %s  %s",
			output.Colorize(output.Green, "#"+strconv.FormatInt(issue.Index, 10)),
			output.Truncate(issue.Title, 60),
			output.Colorize(output.Gray, output.RelativeTimeStr(issue.Updated)),
		))
	}
	if extra := s.Total - len(s.Issues); extra > 0 {
		lines = append(lines, output.Colorize(output.Gray, fmt.Sprintf("And %d more", extra)))
	}
	return lines
}
