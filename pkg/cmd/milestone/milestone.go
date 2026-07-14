package milestone

import (
	"fmt"
	"os"
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
		"invalid due date %q (use YYYY-MM-DD or an RFC3339 timestamp)", v)
}

// dueDateStr formats a milestone's deadline for display. Servers signal "no
// deadline" as null, the zero time, or a far-future sentinel (year 9999).
func dueDateStr(m *forgejo.Milestone) string {
	if m.Deadline == nil || m.Deadline.IsZero() || m.Deadline.Year() >= 9999 {
		return ""
	}
	return m.Deadline.Format("2006-01-02")
}

// webURL returns the milestone's page on the Forgejo web UI. The SDK's
// Milestone has no HTMLURL field, so build it from the repo host.
func webURL(repo cmdutil.Repo, id int64) string {
	scheme := "https"
	if os.Getenv("FJ_INSECURE") != "" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/%s/%s/milestone/%d", scheme, repo.Host, repo.Owner, repo.Name, id)
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
