package api

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Timeline event types, as returned in TimelineComment.Type. Forgejo derives
// these from its internal CommentType enum; only the ones fj renders are
// named here.
const (
	EventComment            = "comment"
	EventReopen             = "reopen"
	EventClose              = "close"
	EventIssueRef           = "issue_ref"
	EventCommitRef          = "commit_ref"
	EventCommentRef         = "comment_ref"
	EventPullRef            = "pull_ref"
	EventLabel              = "label"
	EventMilestone          = "milestone"
	EventAssignees          = "assignees"
	EventChangeTitle        = "change_title"
	EventDeleteBranch       = "delete_branch"
	EventLock               = "lock"
	EventUnlock             = "unlock"
	EventChangeTargetBranch = "change_target_branch"
	EventReviewRequest      = "review_request"
	EventMergePull          = "merge_pull"
	EventDismissReview      = "dismiss_review"
	EventScheduledMerge     = "pull_scheduled_merge"
	EventCancelScheduled    = "pull_cancel_scheduled_merge"
	EventPin                = "pin"
	EventUnpin              = "unpin"
)

// Reference actions, as returned in TimelineComment.RefAction. They describe
// what a reference does to the issue once resolved.
const (
	RefActionNone     = "none"
	RefActionCloses   = "closes"
	RefActionReopens  = "reopens"
	RefActionNeutered = "neutered"
)

// LabelAdded is the Body value Forgejo sets on a "label" event when the label
// was added; removals carry an empty Body.
const LabelAdded = "1"

// TimelineEvent is a single entry in an issue's timeline: either a plain
// comment or an event such as a label change or a commit reference. It
// mirrors the API's TimelineComment schema, limited to the fields fj reads.
//
// Which fields are set depends on Type — RefCommitSHA only on "commit_ref",
// Label only on "label", and so on.
type TimelineEvent struct {
	ID      int64         `json:"id"`
	Type    string        `json:"type"`
	Body    string        `json:"body"`
	Poster  *forgejo.User `json:"user"`
	Created time.Time     `json:"created_at"`
	Updated time.Time     `json:"updated_at"`
	HTMLURL string        `json:"html_url"`

	// RefCommitSHA is the SHA of the commit that referenced this issue.
	RefCommitSHA string         `json:"ref_commit_sha"`
	RefAction    string         `json:"ref_action"`
	RefIssue     *forgejo.Issue `json:"ref_issue"`

	Label           *forgejo.Label     `json:"label"`
	Milestone       *forgejo.Milestone `json:"milestone"`
	OldMilestone    *forgejo.Milestone `json:"old_milestone"`
	Assignee        *forgejo.User      `json:"assignee"`
	RemovedAssignee bool               `json:"removed_assignee"`

	OldTitle string `json:"old_title"`
	NewTitle string `json:"new_title"`
	OldRef   string `json:"old_ref"`
	NewRef   string `json:"new_ref"`
}

// ListIssueTimelineOptions are the filters accepted by the timeline endpoint.
type ListIssueTimelineOptions struct {
	forgejo.ListOptions
	Since  time.Time
	Before time.Time
}

// QueryEncode turns options into querystring argument.
func (opt *ListIssueTimelineOptions) QueryEncode() string {
	query := make(url.Values)
	if opt.Page > 0 {
		query.Add("page", fmt.Sprintf("%d", opt.Page))
	}
	if opt.PageSize > 0 {
		query.Add("limit", fmt.Sprintf("%d", opt.PageSize))
	}
	if !opt.Since.IsZero() {
		query.Add("since", opt.Since.Format(time.RFC3339))
	}
	if !opt.Before.IsZero() {
		query.Add("before", opt.Before.Format(time.RFC3339))
	}
	return query.Encode()
}

// ListIssueTimeline lists comments and events on an issue, oldest first.
// Forgejo omits inline code comments and references the caller cannot see.
//
// Pull requests share the issue index space, so passing a pull request's
// index here returns its timeline.
//
// The SDK has no timeline support at all: ListIssueComments covers a
// different endpoint that returns only plain comments, with no events.
func (c *Client) ListIssueTimeline(owner, repo string, index int64, opt ListIssueTimelineOptions) ([]*TimelineEvent, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/timeline", owner, repo, index)
	if q := opt.QueryEncode(); q != "" {
		path += "?" + q
	}

	var raw []*TimelineEvent
	if err := c.do(http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}

	// Forgejo appends a null when it cannot load an entry's details.
	events := make([]*TimelineEvent, 0, len(raw))
	for _, e := range raw {
		if e != nil {
			events = append(events, e)
		}
	}
	return events, nil
}
