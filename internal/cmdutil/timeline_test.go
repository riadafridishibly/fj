package cmdutil

import (
	"strings"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/api"
)

func user(name string) *forgejo.User { return &forgejo.User{UserName: name} }

// TestEventSummary covers the phrasing of each event type fj renders, and
// the subject swap that makes the same event read correctly on an issue and
// on a pull request.
func TestEventSummary(t *testing.T) {
	tests := []struct {
		name    string
		event   api.TimelineEvent
		subject TimelineSubject
		want    string
	}{
		{
			name:    "commit ref is the motivating case",
			event:   api.TimelineEvent{Type: api.EventCommitRef, RefCommitSHA: "0123456789abcdef"},
			subject: SubjectIssue,
			want:    "referenced this issue from a commit 0123456",
		},
		{
			name:    "commit ref on a pull request says pull request",
			event:   api.TimelineEvent{Type: api.EventCommitRef, RefCommitSHA: "0123456789abcdef"},
			subject: SubjectPull,
			want:    "referenced this pull request from a commit 0123456",
		},
		{
			name:    "short sha is not truncated",
			event:   api.TimelineEvent{Type: api.EventCommitRef, RefCommitSHA: "abc12"},
			subject: SubjectIssue,
			want:    "referenced this issue from a commit abc12",
		},
		{
			name:    "close",
			event:   api.TimelineEvent{Type: api.EventClose},
			subject: SubjectIssue,
			want:    "closed this issue",
		},
		{
			name:    "label added is signalled by body 1",
			event:   api.TimelineEvent{Type: api.EventLabel, Body: api.LabelAdded, Label: &forgejo.Label{Name: "bug"}},
			subject: SubjectIssue,
			want:    "added the bug label",
		},
		{
			name:    "label removed carries an empty body",
			event:   api.TimelineEvent{Type: api.EventLabel, Body: "", Label: &forgejo.Label{Name: "bug"}},
			subject: SubjectIssue,
			want:    "removed the bug label",
		},
		{
			name:    "label event without a label renders nothing",
			event:   api.TimelineEvent{Type: api.EventLabel, Body: api.LabelAdded},
			subject: SubjectIssue,
			want:    "",
		},
		{
			name:    "milestone added",
			event:   api.TimelineEvent{Type: api.EventMilestone, Milestone: &forgejo.Milestone{Title: "v1.0"}},
			subject: SubjectIssue,
			want:    "added this to the v1.0 milestone",
		},
		{
			name:    "milestone removed",
			event:   api.TimelineEvent{Type: api.EventMilestone, OldMilestone: &forgejo.Milestone{Title: "v1.0"}},
			subject: SubjectIssue,
			want:    "removed this from the v1.0 milestone",
		},
		{
			name:    "assigned",
			event:   api.TimelineEvent{Type: api.EventAssignees, Assignee: user("riad")},
			subject: SubjectIssue,
			want:    "assigned riad",
		},
		{
			name:    "unassigned",
			event:   api.TimelineEvent{Type: api.EventAssignees, Assignee: user("riad"), RemovedAssignee: true},
			subject: SubjectIssue,
			want:    "unassigned riad",
		},
		{
			name:    "title change quotes both titles",
			event:   api.TimelineEvent{Type: api.EventChangeTitle, OldTitle: "old", NewTitle: "new"},
			subject: SubjectIssue,
			want:    `changed the title from "old" to "new"`,
		},
		{
			name:    "target branch change",
			event:   api.TimelineEvent{Type: api.EventChangeTargetBranch, OldRef: "main", NewRef: "dev"},
			subject: SubjectPull,
			want:    "changed the target branch from main to dev",
		},
		{
			name:    "merge on a pull request",
			event:   api.TimelineEvent{Type: api.EventMergePull},
			subject: SubjectPull,
			want:    "merged this pull request",
		},
		{
			name:    "plain comments are left to --comments",
			event:   api.TimelineEvent{Type: api.EventComment, Body: "some remark"},
			subject: SubjectIssue,
			want:    "",
		},
		{
			name:    "unknown types are dropped rather than guessed at",
			event:   api.TimelineEvent{Type: "some_future_forgejo_event"},
			subject: SubjectIssue,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventSummary(&tt.event, tt.subject); got != tt.want {
				t.Errorf("eventSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRefActionNote checks that only state-changing references are annotated,
// so an ordinary mention stays unadorned.
func TestRefActionNote(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{api.RefActionCloses, " (closes)"},
		{api.RefActionReopens, " (reopens)"},
		{api.RefActionNone, ""},
		{api.RefActionNeutered, ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := refActionNote(&api.TimelineEvent{Type: api.EventCommitRef, RefAction: tt.action})
			if got != tt.want {
				t.Errorf("refActionNote(%q) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

// TestTimelineLines checks the assembled line: actor, phrasing, ref-action
// annotation and relative time, with unrenderable events dropped so the
// caller's count reflects what is actually printed.
func TestTimelineLines(t *testing.T) {
	events := []*api.TimelineEvent{
		{Type: api.EventComment, Poster: user("riad"), Body: "not a timeline event"},
		{
			Type:         api.EventCommitRef,
			Poster:       user("riad"),
			RefCommitSHA: "0123456789abcdef",
			RefAction:    api.RefActionCloses,
			Created:      time.Now().Add(-5 * time.Hour),
		},
		{Type: "some_future_forgejo_event", Poster: user("riad")},
	}

	lines := TimelineLines(events, SubjectIssue)

	if len(lines) != 1 {
		t.Fatalf("TimelineLines() returned %d lines, want 1: %v", len(lines), lines)
	}
	want := "riad referenced this issue from a commit 0123456 (closes) "
	if !strings.HasPrefix(lines[0], want) {
		t.Errorf("TimelineLines()[0] = %q, want prefix %q", lines[0], want)
	}
	if !strings.Contains(lines[0], "hours ago") {
		t.Errorf("TimelineLines()[0] = %q, want a relative time", lines[0])
	}
}

// TestTimelineLinesUnknownActor keeps an event with no resolvable poster
// visible rather than dropping it or printing an empty name.
func TestTimelineLinesUnknownActor(t *testing.T) {
	events := []*api.TimelineEvent{{Type: api.EventClose, Created: time.Now()}}

	lines := TimelineLines(events, SubjectIssue)

	if len(lines) != 1 {
		t.Fatalf("TimelineLines() returned %d lines, want 1", len(lines))
	}
	if !strings.HasPrefix(lines[0], "unknown closed this issue") {
		t.Errorf("TimelineLines()[0] = %q, want it to start with %q", lines[0], "unknown closed this issue")
	}
}
