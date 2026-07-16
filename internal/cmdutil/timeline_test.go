package cmdutil

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/output"
)

// testRepo is the repository being viewed; references to it stay unqualified.
var testRepo = Repo{Host: "code.example.com", Owner: "example", Name: "project"}

func user(name string) *forgejo.User { return &forgejo.User{UserName: name} }

// refIssue builds the issue a reference came from. pull marks it as a pull
// request, which is how Forgejo signals the source's kind.
func refIssue(index int64, title string, pull bool) *forgejo.Issue {
	issue := &forgejo.Issue{
		Index:      index,
		Title:      title,
		Repository: &forgejo.RepositoryMeta{FullName: testRepo.FullName()},
	}
	if pull {
		issue.PullRequest = &forgejo.PullRequestMeta{}
	}
	return issue
}

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
			name:    "pull ref names which pull request",
			event:   api.TimelineEvent{Type: api.EventPullRef, RefIssue: refIssue(618, "sizing contract", true)},
			subject: SubjectIssue,
			want:    `referenced this issue from pull request #618 "sizing contract"`,
		},
		{
			name:    "issue ref names which issue",
			event:   api.TimelineEvent{Type: api.EventIssueRef, RefIssue: refIssue(626, "typography sweep", false)},
			subject: SubjectIssue,
			want:    `referenced this issue from issue #626 "typography sweep"`,
		},
		{
			// The kind must follow ref_issue, not the event type, so an
			// issue_ref whose source is a pull request still reads right.
			name:    "source kind comes from ref_issue not the event type",
			event:   api.TimelineEvent{Type: api.EventIssueRef, RefIssue: refIssue(618, "sizing contract", true)},
			subject: SubjectIssue,
			want:    `referenced this issue from pull request #618 "sizing contract"`,
		},
		{
			name:    "comment ref names the issue the comment is on",
			event:   api.TimelineEvent{Type: api.EventCommentRef, RefIssue: refIssue(626, "typography sweep", false)},
			subject: SubjectIssue,
			want:    `referenced this issue from a comment on issue #626 "typography sweep"`,
		},
		{
			name:    "unresolved source falls back to a bare phrase",
			event:   api.TimelineEvent{Type: api.EventPullRef},
			subject: SubjectIssue,
			want:    "referenced this issue",
		},
		{
			name:    "unresolved comment source keeps the comment wording",
			event:   api.TimelineEvent{Type: api.EventCommentRef},
			subject: SubjectIssue,
			want:    "referenced this issue from a comment",
		},
		{
			name: "cross-repo reference is qualified with its repository",
			event: api.TimelineEvent{Type: api.EventPullRef, RefIssue: &forgejo.Issue{
				Index:       7,
				Title:       "upstream fix",
				PullRequest: &forgejo.PullRequestMeta{},
				Repository:  &forgejo.RepositoryMeta{FullName: "other/repo"},
			}},
			subject: SubjectIssue,
			want:    `referenced this issue from pull request other/repo#7 "upstream fix"`,
		},
		{
			name:    "a reference with no title falls back to the number",
			event:   api.TimelineEvent{Type: api.EventPullRef, RefIssue: refIssue(618, "", true)},
			subject: SubjectIssue,
			want:    "referenced this issue from pull request #618",
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
			// Compose phrase and title the way an unbounded terminal would,
			// so each case reads as the line the user sees.
			ev := eventSummary(&tt.event, tt.subject, testRepo)
			got := ev.phrase
			if ev.title != "" {
				got = fmt.Sprintf("%s %q", ev.phrase, ev.title)
			}
			if got != tt.want {
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

	lines := TimelineLines(events, SubjectIssue, testRepo)

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

// TestTimelineLinesFitsTerminalWidth checks that a referencing title takes
// whatever columns the rest of the line leaves, so a wide terminal shows more
// of it than a narrow one and neither overflows.
func TestTimelineLinesFitsTerminalWidth(t *testing.T) {
	longTitle := "feat(trustcard-dashboard): sizing contract — control-size vocabulary, type-scale floor, drift ratchet"
	events := []*api.TimelineEvent{{
		Type:     api.EventPullRef,
		Poster:   user("riad"),
		RefIssue: refIssue(618, longTitle, true),
		Created:  time.Now(),
	}}

	for _, width := range []int{80, 120, 200} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			t.Setenv("FJ_WIDTH", strconv.Itoa(width))

			lines := TimelineLines(events, SubjectIssue, testRepo)
			if len(lines) != 1 {
				t.Fatalf("got %d lines, want 1", len(lines))
			}
			if got := output.DisplayWidth(lines[0]); got > width {
				t.Errorf("line is %d columns wide, want at most %d:\n%s", got, width, lines[0])
			}
			if !strings.Contains(lines[0], "pull request #618") {
				t.Errorf("the reference itself must survive any width:\n%s", lines[0])
			}
		})
	}
}

// TestTimelineLinesNarrowTerminal covers a terminal too narrow even for the
// phrase. The title is dropped, and what remains is the reference itself: the
// phrase is fixed, so like a table's non-flexible columns it cannot shrink
// further and is allowed to overflow rather than lose the event.
func TestTimelineLinesNarrowTerminal(t *testing.T) {
	t.Setenv("FJ_WIDTH", "40")

	events := []*api.TimelineEvent{{
		Type:     api.EventPullRef,
		Poster:   user("riad"),
		RefIssue: refIssue(618, "sizing contract", true),
		Created:  time.Now(),
	}}

	line := TimelineLines(events, SubjectIssue, testRepo)[0]

	if strings.Contains(line, `"`) {
		t.Errorf("a title cannot fit at width 40 and should be dropped:\n%s", line)
	}
	if !strings.Contains(line, "pull request #618") {
		t.Errorf("the reference must survive even when the title cannot:\n%s", line)
	}
}

// TestTimelineLinesWiderTerminalShowsMoreTitle pins the actual point of
// fitting to the terminal: width buys title.
func TestTimelineLinesWiderTerminalShowsMoreTitle(t *testing.T) {
	events := []*api.TimelineEvent{{
		Type:     api.EventPullRef,
		Poster:   user("riad"),
		RefIssue: refIssue(618, strings.Repeat("long title ", 20), true),
		Created:  time.Now(),
	}}

	t.Setenv("FJ_WIDTH", "80")
	narrow := TimelineLines(events, SubjectIssue, testRepo)[0]
	t.Setenv("FJ_WIDTH", "160")
	wide := TimelineLines(events, SubjectIssue, testRepo)[0]

	if !(output.DisplayWidth(wide) > output.DisplayWidth(narrow)) {
		t.Errorf("a wider terminal should show more title:\nnarrow: %s\nwide:   %s", narrow, wide)
	}
}

// TestTimelineLinesUnboundedKeepsWholeTitle covers the piped case: with no
// terminal there is no budget, and a downstream tool or agent must receive the
// title intact rather than an ellipsis.
func TestTimelineLinesUnboundedKeepsWholeTitle(t *testing.T) {
	title := "feat(trustcard-dashboard): sizing contract — control-size vocabulary, type-scale floor, drift ratchet"
	events := []*api.TimelineEvent{{
		Type:     api.EventPullRef,
		Poster:   user("riad"),
		RefIssue: refIssue(618, title, true),
		Created:  time.Now(),
	}}

	t.Setenv("FJ_WIDTH", "0") // falls through to the non-TTY default of unbounded

	lines := TimelineLines(events, SubjectIssue, testRepo)
	if !strings.Contains(lines[0], title) {
		t.Errorf("an unbounded line must keep the whole title:\n%s", lines[0])
	}
	if strings.Contains(lines[0], "…") {
		t.Errorf("an unbounded line must not be truncated:\n%s", lines[0])
	}
}

// TestFitEventLine covers the budget arithmetic directly, including the floor
// below which a title is dropped rather than rendered as a stub.
func TestFitEventLine(t *testing.T) {
	tests := []struct {
		name  string
		head  string
		title string
		tail  string
		width int
		want  string
	}{
		{
			name:  "unbounded keeps the title whole",
			head:  "riad referenced this issue from pull request #618",
			title: "sizing contract",
			tail:  " 5 hours ago",
			width: 0,
			want:  `riad referenced this issue from pull request #618 "sizing contract" 5 hours ago`,
		},
		{
			name:  "a title that fits is untouched",
			head:  "riad referenced this issue from pull request #618",
			title: "sizing contract",
			tail:  " 5 hours ago",
			width: 120,
			want:  `riad referenced this issue from pull request #618 "sizing contract" 5 hours ago`,
		},
		{
			name:  "no title renders head and tail alone",
			head:  "riad closed this issue",
			title: "",
			tail:  " 5 hours ago",
			width: 80,
			want:  "riad closed this issue 5 hours ago",
		},
		{
			name:  "a title squeezed below the floor is dropped, not stubbed",
			head:  "riad referenced this issue from pull request #618",
			title: "sizing contract",
			tail:  " (closes) 5 hours ago",
			width: 75,
			want:  "riad referenced this issue from pull request #618 (closes) 5 hours ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fitEventLine(tt.head, tt.title, tt.tail, tt.width); got != tt.want {
				t.Errorf("fitEventLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTimelineLinesUnknownActor keeps an event with no resolvable poster
// visible rather than dropping it or printing an empty name.
func TestTimelineLinesUnknownActor(t *testing.T) {
	events := []*api.TimelineEvent{{Type: api.EventClose, Created: time.Now()}}

	lines := TimelineLines(events, SubjectIssue, testRepo)

	if len(lines) != 1 {
		t.Fatalf("TimelineLines() returned %d lines, want 1", len(lines))
	}
	if !strings.HasPrefix(lines[0], "unknown closed this issue") {
		t.Errorf("TimelineLines()[0] = %q, want it to start with %q", lines[0], "unknown closed this issue")
	}
}
