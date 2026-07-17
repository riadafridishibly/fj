package cmdutil

import (
	"errors"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/api"
)

// renderedEvents is one well-formed event of every type fj phrases, used to
// hold eventSummary and eventCategory to the same set of types.
var renderedEvents = []api.TimelineEvent{
	{Type: api.EventCommitRef, RefCommitSHA: "0123456789abcdef"},
	{Type: api.EventIssueRef, RefIssue: refIssue(1, "an issue", false)},
	{Type: api.EventPullRef, RefIssue: refIssue(2, "a pull request", true)},
	{Type: api.EventCommentRef, RefIssue: refIssue(3, "a comment source", false)},
	{Type: api.EventClose},
	{Type: api.EventReopen},
	{Type: api.EventMergePull},
	{Type: api.EventLock},
	{Type: api.EventUnlock},
	{Type: api.EventPin},
	{Type: api.EventUnpin},
	{Type: api.EventLabel, Label: &forgejo.Label{Name: "bug"}, Body: api.LabelAdded},
	{Type: api.EventMilestone, Milestone: &forgejo.Milestone{Title: "v1"}},
	{Type: api.EventAssignees, Assignee: user("riad")},
	{Type: api.EventChangeTitle, OldTitle: "old", NewTitle: "new"},
	{Type: api.EventDeleteBranch, OldRef: "feat/x"},
	{Type: api.EventChangeTargetBranch, OldRef: "main", NewRef: "dev"},
	{Type: api.EventReviewRequest},
	{Type: api.EventDismissReview},
	{Type: api.EventScheduledMerge},
	{Type: api.EventCancelScheduled},
}

// TestRenderedEventsHaveACategory guards the two switches against drifting
// apart. An event fj phrases but cannot categorise would vanish from any
// --timeline-include, which is the failure this catches.
func TestRenderedEventsHaveACategory(t *testing.T) {
	for _, event := range renderedEvents {
		t.Run(event.Type, func(t *testing.T) {
			if eventSummary(&event, SubjectIssue, testRepo).phrase == "" {
				t.Fatalf("fixture for %q renders no phrase, so it proves nothing", event.Type)
			}
			if got := eventCategory(&event); got == "" {
				t.Errorf("%q renders but has no category: --timeline-include would drop it", event.Type)
			}
		})
	}
}

// TestRefCategoryUsesSourceNotType covers the reason the filter exists in this
// shape: Forgejo records comment_ref for a comment on an issue and on a pull
// request alike, so the type cannot say which a reference came from.
func TestRefCategoryUsesSourceNotType(t *testing.T) {
	tests := []struct {
		name  string
		event api.TimelineEvent
		want  TimelineCategory
	}{
		{
			name:  "commit ref is its own source",
			event: api.TimelineEvent{Type: api.EventCommitRef, RefCommitSHA: "abc1234"},
			want:  CategoryCommits,
		},
		{
			name:  "comment on an issue counts as an issue",
			event: api.TimelineEvent{Type: api.EventCommentRef, RefIssue: refIssue(1, "t", false)},
			want:  CategoryIssues,
		},
		{
			name:  "comment on a pull request counts as a pull request",
			event: api.TimelineEvent{Type: api.EventCommentRef, RefIssue: refIssue(2, "t", true)},
			want:  CategoryPulls,
		},
		{
			name:  "issue ref body reference",
			event: api.TimelineEvent{Type: api.EventIssueRef, RefIssue: refIssue(3, "t", false)},
			want:  CategoryIssues,
		},
		{
			name:  "pull ref body reference",
			event: api.TimelineEvent{Type: api.EventPullRef, RefIssue: refIssue(4, "t", true)},
			want:  CategoryPulls,
		},
		{
			name:  "unresolved pull ref falls back to the type",
			event: api.TimelineEvent{Type: api.EventPullRef},
			want:  CategoryPulls,
		},
		{
			name:  "unresolved issue ref falls back to the type",
			event: api.TimelineEvent{Type: api.EventIssueRef},
			want:  CategoryIssues,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventCategory(&tt.event); got != tt.want {
				t.Errorf("eventCategory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewTimelineFilterRejectsUnknownNames(t *testing.T) {
	_, err := NewTimelineFilter(nil, []string{"commit"})
	if err == nil {
		t.Fatal("expected an error for an unknown category, got nil")
	}
	if !strings.Contains(err.Error(), "--timeline-exclude") {
		t.Errorf("error should name the flag, got: %v", err)
	}
	if !strings.Contains(err.Error(), "commits") {
		t.Errorf("error should list the valid names, got: %v", err)
	}
	var flagErr *FlagError
	if !errors.As(err, &flagErr) {
		t.Errorf("expected a FlagError so usage is shown, got %T", err)
	}
}

func TestTimelineFilter(t *testing.T) {
	events := []*api.TimelineEvent{
		{Type: api.EventCommitRef, RefCommitSHA: "abc1234"},
		{Type: api.EventPullRef, RefIssue: refIssue(1, "a pr", true)},
		{Type: api.EventCommentRef, RefIssue: refIssue(2, "an issue", false)},
		{Type: api.EventLabel, Label: &forgejo.Label{Name: "bug"}, Body: api.LabelAdded},
		{Type: api.EventComment}, // no category: only --json ever shows it
	}

	tests := []struct {
		name    string
		include []string
		exclude []string
		want    []string
	}{
		{
			name: "no filter keeps everything",
			want: []string{api.EventCommitRef, api.EventPullRef, api.EventCommentRef, api.EventLabel, api.EventComment},
		},
		{
			name:    "excluding commits is the motivating case",
			exclude: []string{"commits"},
			want:    []string{api.EventPullRef, api.EventCommentRef, api.EventLabel, api.EventComment},
		},
		{
			name:    "excluding prs drops a pr reference made in a comment",
			exclude: []string{"prs"},
			want:    []string{api.EventCommitRef, api.EventCommentRef, api.EventLabel, api.EventComment},
		},
		{
			name:    "include narrows and drops uncategorised events",
			include: []string{"labels"},
			want:    []string{api.EventLabel},
		},
		{
			name:    "the refs group stands for every source",
			include: []string{"refs"},
			want:    []string{api.EventCommitRef, api.EventPullRef, api.EventCommentRef},
		},
		{
			name:    "exclude subtracts from include",
			include: []string{"refs"},
			exclude: []string{"commits"},
			want:    []string{api.EventPullRef, api.EventCommentRef},
		},
		{
			name:    "names are case and space insensitive",
			exclude: []string{" Commits "},
			want:    []string{api.EventPullRef, api.EventCommentRef, api.EventLabel, api.EventComment},
		},
		{
			name:    "an empty value is not a filter",
			exclude: []string{""},
			want:    []string{api.EventCommitRef, api.EventPullRef, api.EventCommentRef, api.EventLabel, api.EventComment},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewTimelineFilter(tt.include, tt.exclude)
			if err != nil {
				t.Fatalf("NewTimelineFilter() error = %v", err)
			}
			var got []string
			for _, e := range filter.Apply(events) {
				got = append(got, e.Type)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("kept %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTimelineFilterValues keeps the help text honest: every name the flags
// accept must be listed for a reader to discover it.
func TestTimelineFilterValues(t *testing.T) {
	values := TimelineFilterValues()
	for _, cat := range timelineCategories {
		if !contains(values, string(cat)) {
			t.Errorf("category %q is accepted but missing from the help", cat)
		}
	}
	for group := range timelineGroups {
		if !contains(values, group) {
			t.Errorf("group %q is accepted but missing from the help", group)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
