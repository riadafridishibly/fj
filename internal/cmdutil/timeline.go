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

// titleFloor is the narrowest a referencing title may be squeezed before it
// is dropped: below this it is more ellipsis than information. It mirrors the
// floor a Flexible table column shrinks to.
const titleFloor = 12

// eventLine is one event ready to render. The title of a referencing issue is
// held apart from the phrase because it is the only part that can shrink, the
// way a Flexible table column yields width while fixed columns do not.
type eventLine struct {
	// phrase reads as a complete event on its own, e.g. "referenced this
	// issue from pull request #618".
	phrase string
	// title is quoted after phrase when it survives the width budget.
	title string
}

// TimelineLines renders one line per event fj has phrasing for, oldest
// first. Events it cannot phrase are dropped, so the count of lines is not
// the count of events; --json exposes the raw timeline either way.
//
// repo is the repository being viewed, used to leave same-repo references
// unqualified while spelling out cross-repo ones.
//
// Lines are fitted to the terminal, so a wide terminal shows more of a
// referencing title than a narrow one. When stdout is not a terminal the
// budget is unbounded and titles are kept whole, which is what a pipe into
// jq or an agent wants.
func TimelineLines(events []*api.TimelineEvent, subject TimelineSubject, repo Repo) []string {
	width := output.TerminalWidth()

	lines := make([]string, 0, len(events))
	for _, e := range events {
		ev := eventSummary(e, subject, repo)
		if ev.phrase == "" {
			continue
		}
		head := eventActor(e) + " " + ev.phrase
		tail := refActionNote(e) + " " + output.RelativeTimeStr(e.Created)
		lines = append(lines, fitEventLine(head, ev.title, tail, width))
	}
	return lines
}

// fitEventLine assembles a line, giving the title whatever columns the rest of
// the line leaves. A width of 0 means unbounded. A title squeezed below
// titleFloor is dropped rather than rendered as a stub, since the phrase
// already names the reference by number.
func fitEventLine(head, title, tail string, width int) string {
	if title == "" {
		return head + tail
	}
	if width > 0 {
		// The title costs a leading space and two quotes beyond its own width.
		budget := width - output.DisplayWidth(head) - output.DisplayWidth(tail) - 3
		if budget < titleFloor {
			return head + tail
		}
		title = output.TruncateDisplay(title, budget)
	}
	return fmt.Sprintf("%s %q%s", head, title, tail)
}

// refSource names where a reference came from, as "pull request #618", with
// the referencing title returned separately. Both are "" when Forgejo did not
// resolve the source, leaving the caller to fall back to a bare phrase.
//
// The kind is taken from the referencing issue itself rather than from the
// event type: ref_issue carries a PullRequest only for a pull request, which
// is the same signal Forgejo's own UI uses.
func refSource(e *api.TimelineEvent, repo Repo) (source, title string) {
	if e.RefIssue == nil {
		return "", ""
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

	return kind + " " + ref, output.Sanitize(e.RefIssue.Title)
}

// eventSummary renders an event as a phrase completing "<user> ...". Its
// phrase is "" for entries carrying no event of their own: plain comments,
// which --comments prints in full, and types fj has no phrasing for.
func eventSummary(e *api.TimelineEvent, subject TimelineSubject, repo Repo) eventLine {
	this := "this " + string(subject)

	switch e.Type {
	case api.EventCommitRef:
		return eventLine{phrase: fmt.Sprintf("referenced %s from a commit %s", this, shortSHA(e.RefCommitSHA))}
	case api.EventIssueRef, api.EventPullRef:
		src, title := refSource(e, repo)
		if src == "" {
			return eventLine{phrase: "referenced " + this}
		}
		return eventLine{phrase: "referenced " + this + " from " + src, title: title}
	case api.EventCommentRef:
		src, title := refSource(e, repo)
		if src == "" {
			return eventLine{phrase: "referenced " + this + " from a comment"}
		}
		return eventLine{phrase: "referenced " + this + " from a comment on " + src, title: title}
	case api.EventClose:
		return eventLine{phrase: "closed " + this}
	case api.EventReopen:
		return eventLine{phrase: "reopened " + this}
	case api.EventMergePull:
		return eventLine{phrase: "merged " + this}
	case api.EventLock:
		return eventLine{phrase: "locked " + this}
	case api.EventUnlock:
		return eventLine{phrase: "unlocked " + this}
	case api.EventPin:
		return eventLine{phrase: "pinned " + this}
	case api.EventUnpin:
		return eventLine{phrase: "unpinned " + this}
	case api.EventLabel:
		return eventLine{phrase: labelSummary(e)}
	case api.EventMilestone:
		return eventLine{phrase: milestoneSummary(e)}
	case api.EventAssignees:
		return eventLine{phrase: assigneeSummary(e)}
	case api.EventChangeTitle:
		return eventLine{phrase: fmt.Sprintf("changed the title from %q to %q", e.OldTitle, e.NewTitle)}
	case api.EventDeleteBranch:
		if e.OldRef == "" {
			return eventLine{phrase: "deleted the branch"}
		}
		return eventLine{phrase: "deleted the " + e.OldRef + " branch"}
	case api.EventChangeTargetBranch:
		return eventLine{phrase: fmt.Sprintf("changed the target branch from %s to %s", e.OldRef, e.NewRef)}
	case api.EventReviewRequest:
		return eventLine{phrase: "requested a review"}
	case api.EventDismissReview:
		return eventLine{phrase: "dismissed a review"}
	case api.EventScheduledMerge:
		return eventLine{phrase: "scheduled " + this + " to auto-merge"}
	case api.EventCancelScheduled:
		return eventLine{phrase: "cancelled the scheduled auto-merge"}
	default:
		return eventLine{}
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
