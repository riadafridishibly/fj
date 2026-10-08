package cmdutil

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/api"
)

// TimelineCategory groups timeline events by what they mean, which is not the
// same as the type Forgejo sends. Filtering is expressed in these terms so
// that "hide pull request references" means what a reader expects, whichever
// wire type carried the reference.
type TimelineCategory string

const (
	// CategoryCommits and its siblings cover references: something pointed at
	// this issue. They are named for the source, not for the event type.
	CategoryCommits TimelineCategory = "commits"
	CategoryIssues  TimelineCategory = "issues"
	CategoryPulls   TimelineCategory = "prs"

	CategoryLabels     TimelineCategory = "labels"
	CategoryMilestones TimelineCategory = "milestones"
	CategoryAssignees  TimelineCategory = "assignees"
	CategoryState      TimelineCategory = "state"
	CategoryTitle      TimelineCategory = "title"
	CategoryBranches   TimelineCategory = "branches"
	CategoryReviews    TimelineCategory = "reviews"
	CategoryLocks      TimelineCategory = "locks"
	CategoryPins       TimelineCategory = "pins"
)

var timelineCategories = []TimelineCategory{
	CategoryCommits, CategoryIssues, CategoryPulls,
	CategoryLabels, CategoryMilestones, CategoryAssignees,
	CategoryState, CategoryTitle, CategoryBranches,
	CategoryReviews, CategoryLocks, CategoryPins,
}

// timelineGroups are shorthands that stand for several categories, so a
// reader can ask for every reference without naming each source.
var timelineGroups = map[string][]TimelineCategory{
	"refs": {CategoryCommits, CategoryIssues, CategoryPulls},
}

// eventCategory groups an event for filtering. It returns "" for events fj has
// no phrasing for, which the text output drops in any case.
func eventCategory(e *api.TimelineEvent) TimelineCategory {
	switch e.Type {
	case api.EventCommitRef, api.EventIssueRef, api.EventPullRef, api.EventCommentRef:
		return refCategory(e)
	case api.EventClose, api.EventReopen, api.EventMergePull,
		api.EventScheduledMerge, api.EventCancelScheduled:
		return CategoryState
	case api.EventLabel:
		return CategoryLabels
	case api.EventMilestone:
		return CategoryMilestones
	case api.EventAssignees:
		return CategoryAssignees
	case api.EventChangeTitle:
		return CategoryTitle
	case api.EventDeleteBranch, api.EventChangeTargetBranch:
		return CategoryBranches
	case api.EventReviewRequest, api.EventDismissReview:
		return CategoryReviews
	case api.EventLock, api.EventUnlock:
		return CategoryLocks
	case api.EventPin, api.EventUnpin:
		return CategoryPins
	default:
		return ""
	}
}

// refCategory groups a reference by what it came from.
//
// The event type cannot answer that on its own. Forgejo records comment_ref
// for a comment on an issue and for a comment on a pull request alike, so only
// ref_issue distinguishes them; keying on the type would leak pull request
// references into a filter that asked for issues. The type is a fallback for a
// reference Forgejo did not resolve, where it still holds: issue_ref and
// pull_ref are chosen from the source's own kind.
func refCategory(e *api.TimelineEvent) TimelineCategory {
	if e.Type == api.EventCommitRef {
		return CategoryCommits
	}
	switch refKind(e) {
	case kindPull:
		return CategoryPulls
	case kindIssue:
		return CategoryIssues
	}
	if e.Type == api.EventPullRef {
		return CategoryPulls
	}
	return CategoryIssues
}

// TimelineFilter selects which timeline events to keep. The zero value keeps
// every event, which is what a view with no filter flags wants.
type TimelineFilter struct {
	// include is nil unless the reader narrowed the timeline, so that "not
	// asked for" stays distinct from "asked for nothing".
	include map[TimelineCategory]bool
	exclude map[TimelineCategory]bool
}

// ParseTimelineFilter builds a filter from --timeline. Plain names narrow to
// those categories, "-" names drop theirs: "refs,-commits" keeps issue and pull
// request references. "all" narrows nothing.
func ParseTimelineFilter(terms []string) (TimelineFilter, error) {
	var include, exclude []string
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if name, ok := strings.CutPrefix(t, "-"); ok {
			if name == "" {
				return TimelineFilter{}, FlagErrorf("invalid --timeline value %q: name a category after -", t)
			}
			exclude = append(exclude, name)
		} else if !strings.EqualFold(t, "all") {
			include = append(include, t)
		}
	}
	inc, err := resolveCategories(include)
	if err != nil {
		return TimelineFilter{}, err
	}
	exc, err := resolveCategories(exclude)
	if err != nil {
		return TimelineFilter{}, err
	}
	return TimelineFilter{include: inc, exclude: exc}, nil
}

// Allows reports whether an event survives the filter.
//
// An event fj has no category for has no way to match an include, so narrowing
// drops it; excluding a category leaves it alone. That keeps "-" names
// subtractive over the whole timeline, including the raw types only --json
// shows.
func (f TimelineFilter) Allows(e *api.TimelineEvent) bool {
	cat := eventCategory(e)
	if f.include != nil && !f.include[cat] {
		return false
	}
	return !f.exclude[cat]
}

// Apply drops the events the filter excludes, preserving order. It returns the
// events untouched when no filter was given, so an unfiltered --json stays a
// faithful copy of what Forgejo sent.
func (f TimelineFilter) Apply(events []*api.TimelineEvent) []*api.TimelineEvent {
	if f.include == nil && f.exclude == nil {
		return events
	}
	kept := make([]*api.TimelineEvent, 0, len(events))
	for _, e := range events {
		if f.Allows(e) {
			kept = append(kept, e)
		}
	}
	return kept
}

// resolveCategories turns flag values into a category set, expanding group
// shorthands. It returns nil for an empty selection so the caller can tell it
// apart from a selection of nothing.
func resolveCategories(names []string) (map[TimelineCategory]bool, error) {
	set := make(map[TimelineCategory]bool)
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if group, ok := timelineGroups[name]; ok {
			for _, c := range group {
				set[c] = true
			}
			continue
		}
		cat := TimelineCategory(name)
		if !validCategory(cat) {
			return nil, FlagErrorf(
				"invalid --timeline value %q (valid: %s)",
				name, strings.Join(TimelineFilterValues(), ", "),
			)
		}
		set[cat] = true
	}
	if len(set) == 0 {
		return nil, nil
	}
	return set, nil
}

func validCategory(c TimelineCategory) bool {
	return slices.Contains(timelineCategories, c)
}

// AddTimelineFlag adds --timeline to a view. terms stays nil unless it is given.
func AddTimelineFlag(cmd *cobra.Command, terms *[]string, subject TimelineSubject) {
	cmd.Flags().StringSliceVar(terms, "timeline", nil, fmt.Sprintf(
		"Show %s events: all, or the `categories` named; prefix one with - to hide it (%s)",
		subject, strings.Join(TimelineFilterValues(), ", ")))
}

// TimelineFilterValues returns every name the --timeline flag accepts,
// sorted for stable help and error output.
func TimelineFilterValues() []string {
	vals := make([]string, 0, len(timelineCategories)+len(timelineGroups))
	for _, c := range timelineCategories {
		vals = append(vals, string(c))
	}
	for g := range timelineGroups {
		vals = append(vals, g)
	}
	sort.Strings(vals)
	return vals
}
