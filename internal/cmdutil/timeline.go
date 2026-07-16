package cmdutil

import (
	"fmt"
	"strings"

	"github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/output"
)

// TimelineSubject is what a timeline hangs off, so events read "closed this
// issue" or "closed this pull request" the way Forgejo words them.
type TimelineSubject string

const (
	SubjectIssue TimelineSubject = "issue"
	SubjectPull  TimelineSubject = "pull request"
)

// Timeline lists the events on an issue or pull request. Forgejo gives both
// a shared index space, so pull requests use the issue endpoint too.
func (f *Factory) Timeline(repo Repo, index int64) ([]*api.TimelineEvent, error) {
	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	events, err := client.ListIssueTimeline(repo.Owner, repo.Name, index, api.ListIssueTimelineOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing timeline: %w", err)
	}
	return events, nil
}

// TimelineLines renders one line per event fj has phrasing for, oldest
// first. Events it cannot phrase are dropped, so the count of lines is not
// the count of events; --json exposes the raw timeline either way.
//
// repo is the repository being viewed, used to leave same-repo references
// unqualified while spelling out cross-repo ones.
func TimelineLines(events []*api.TimelineEvent, subject TimelineSubject, repo Repo) []string {
	lines := make([]string, 0, len(events))
	for _, e := range events {
		summary := eventSummary(e, subject, repo)
		if summary == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s%s %s",
			eventActor(e), summary, refActionNote(e), output.RelativeTimeStr(e.Created)))
	}
	return lines
}

// refTitleWidth caps how much of a referencing issue's title is shown. Real
// titles routinely run past 100 characters, which would bury the event.
const refTitleWidth = 60

// refSource names where a reference came from, as "pull request #618
// \"sizing contract…\"". It returns "" when Forgejo did not resolve the
// source, leaving the caller to fall back to a bare phrase.
//
// The kind is taken from the referencing issue itself rather than from the
// event type: ref_issue carries a PullRequest only for a pull request, which
// is the same signal Forgejo's own UI uses.
func refSource(e *api.TimelineEvent, repo Repo) string {
	if e.RefIssue == nil {
		return ""
	}

	kind := "issue"
	if e.RefIssue.PullRequest != nil {
		kind = "pull request"
	}

	ref := fmt.Sprintf("#%d", e.RefIssue.Index)
	// A cross-repo reference needs its repository spelled out, or "#12"
	// would read as this repository's #12.
	if r := e.RefIssue.Repository; r != nil && r.FullName != "" && !strings.EqualFold(r.FullName, repo.FullName()) {
		ref = r.FullName + ref
	}

	title := output.TruncateDisplay(output.Sanitize(e.RefIssue.Title), refTitleWidth)
	if title == "" {
		return kind + " " + ref
	}
	return fmt.Sprintf("%s %s %q", kind, ref, title)
}

// eventSummary renders an event as a phrase completing "<user> ...". It
// returns "" for entries carrying no event of their own: plain comments,
// which --comments prints in full, and types fj has no phrasing for.
func eventSummary(e *api.TimelineEvent, subject TimelineSubject, repo Repo) string {
	this := "this " + string(subject)

	switch e.Type {
	case api.EventCommitRef:
		return fmt.Sprintf("referenced %s from a commit %s", this, shortSHA(e.RefCommitSHA))
	case api.EventIssueRef, api.EventPullRef:
		if src := refSource(e, repo); src != "" {
			return "referenced " + this + " from " + src
		}
		return "referenced " + this
	case api.EventCommentRef:
		if src := refSource(e, repo); src != "" {
			return "referenced " + this + " from a comment on " + src
		}
		return "referenced " + this + " from a comment"
	case api.EventClose:
		return "closed " + this
	case api.EventReopen:
		return "reopened " + this
	case api.EventMergePull:
		return "merged " + this
	case api.EventLock:
		return "locked " + this
	case api.EventUnlock:
		return "unlocked " + this
	case api.EventPin:
		return "pinned " + this
	case api.EventUnpin:
		return "unpinned " + this
	case api.EventLabel:
		return labelSummary(e)
	case api.EventMilestone:
		return milestoneSummary(e)
	case api.EventAssignees:
		return assigneeSummary(e)
	case api.EventChangeTitle:
		return fmt.Sprintf("changed the title from %q to %q", e.OldTitle, e.NewTitle)
	case api.EventDeleteBranch:
		if e.OldRef == "" {
			return "deleted the branch"
		}
		return "deleted the " + e.OldRef + " branch"
	case api.EventChangeTargetBranch:
		return fmt.Sprintf("changed the target branch from %s to %s", e.OldRef, e.NewRef)
	case api.EventReviewRequest:
		return "requested a review"
	case api.EventDismissReview:
		return "dismissed a review"
	case api.EventScheduledMerge:
		return "scheduled " + this + " to auto-merge"
	case api.EventCancelScheduled:
		return "cancelled the scheduled auto-merge"
	default:
		return ""
	}
}

func labelSummary(e *api.TimelineEvent) string {
	if e.Label == nil {
		return ""
	}
	if e.Body == api.LabelAdded {
		return "added the " + e.Label.Name + " label"
	}
	return "removed the " + e.Label.Name + " label"
}

func milestoneSummary(e *api.TimelineEvent) string {
	switch {
	case e.Milestone != nil:
		return "added this to the " + e.Milestone.Title + " milestone"
	case e.OldMilestone != nil:
		return "removed this from the " + e.OldMilestone.Title + " milestone"
	default:
		return "changed the milestone"
	}
}

func assigneeSummary(e *api.TimelineEvent) string {
	if e.Assignee == nil {
		return ""
	}
	if e.RemovedAssignee {
		return "unassigned " + e.Assignee.UserName
	}
	return "assigned " + e.Assignee.UserName
}

// refActionNote marks a reference that changes state once resolved, so it
// reads "referenced this issue from a commit abc1234 (closes)".
func refActionNote(e *api.TimelineEvent) string {
	switch e.RefAction {
	case api.RefActionCloses, api.RefActionReopens:
		return " (" + e.RefAction + ")"
	default:
		return ""
	}
}

// eventActor names who triggered an event, falling back the way view
// commands do for an unresolvable poster.
func eventActor(e *api.TimelineEvent) string {
	if e.Poster != nil && e.Poster.UserName != "" {
		return e.Poster.UserName
	}
	return "unknown"
}

// shortSHA abbreviates a commit SHA to the 7 characters Forgejo displays.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
