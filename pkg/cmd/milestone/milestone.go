package milestone

import (
	"fmt"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdMilestone(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "milestone <command>",
		Short:   "Manage milestones",
		Long:    "Work with Forgejo repository milestones.",
		Aliases: []string{"ms"},
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdEdit(f))
	cmd.AddCommand(NewCmdClose(f))
	cmd.AddCommand(NewCmdReopen(f))
	cmd.AddCommand(NewCmdDelete(f))

	return cmd
}

// parseDueDate parses a --due-date value as YYYY-MM-DD or RFC3339.
func parseDueDate(v string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	return time.Time{}, cmdutil.FlagErrorf(
		"invalid due date %q (use YYYY-MM-DD or an RFC3339 timestamp)", v,
	)
}

// dueDateStr formats a milestone's deadline for display, or "" when it has
// none.
func dueDateStr(m *forgejo.Milestone) string {
	if d := cmdutil.MilestoneDeadline(m); d != nil {
		return d.Format("2006-01-02")
	}
	return ""
}

// webURL returns the milestone's page on the Forgejo web UI. The SDK's
// Milestone has no HTMLURL field, so build it from the repo host.
func webURL(repo cmdutil.Repo, id int64) string {
	return fmt.Sprintf("%s/%s/%s/milestone/%d", cmdutil.BaseURL(repo.Host), repo.Owner, repo.Name, id)
}

// milestoneFields are gh's milestone shape, as in gh issue view --json
// milestone. gh has no milestone commands; milestoneFJFields are the rest of
// what Forgejo carries.
var (
	milestoneFields   = []string{"description", "dueOn", "number", "title"}
	milestoneFJFields = []string{"closedAt", "closedIssues", "createdAt", "openIssues", "state", "updatedAt", "url"}
)

// milestoneJSON returns m keyed by field name.
func milestoneJSON(repo cmdutil.Repo, m *forgejo.Milestone) map[string]any {
	out := cmdutil.JSONMilestone(m)
	out["closedAt"] = cmdutil.JSONTime(m.Closed)
	out["closedIssues"] = m.ClosedIssues
	out["createdAt"] = cmdutil.JSONTime(&m.Created)
	out["openIssues"] = m.OpenIssues
	out["state"] = strings.ToUpper(string(m.State))
	out["updatedAt"] = cmdutil.JSONTime(m.Updated)
	out["url"] = webURL(repo, m.ID)
	return out
}

// setState transitions a milestone to the given state. Shared by close and
// reopen.
func setState(f *cmdutil.Factory, name string, state forgejo.StateType) (*forgejo.Milestone, error) {
	repo, err := f.BaseRepo()
	if err != nil {
		return nil, err
	}

	client, err := f.ClientForRepo(repo)
	if err != nil {
		return nil, err
	}

	ms, _, err := client.EditMilestoneByName(repo.Owner, repo.Name, name, forgejo.EditMilestoneOption{
		State: &state,
	})
	return ms, err
}
