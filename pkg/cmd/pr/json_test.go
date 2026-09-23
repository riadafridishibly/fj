package pr

import (
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func TestPRJSONState(t *testing.T) {
	tests := []struct {
		name                     string
		state                    forgejo.StateType
		merged, mergeable, draft bool
		headRepo                 int64
		wantState, wantMergeable string
		wantClosed, wantCross    bool
	}{
		{"open mergeable", forgejo.StateOpen, false, true, false, 1, "OPEN", "MERGEABLE", false, false},
		{"open conflicting", forgejo.StateOpen, false, false, false, 1, "OPEN", "CONFLICTING", false, false},
		{"draft", forgejo.StateOpen, false, false, true, 1, "OPEN", "UNKNOWN", false, false},
		{"closed", forgejo.StateClosed, false, false, false, 1, "CLOSED", "UNKNOWN", true, false},
		{"merged", forgejo.StateClosed, true, true, false, 1, "MERGED", "UNKNOWN", true, false},
		{"fork", forgejo.StateOpen, false, true, false, 2, "OPEN", "MERGEABLE", false, true},
	}
	for _, tt := range tests {
		pr := &apiPullRequest{Draft: tt.draft, PullRequest: forgejo.PullRequest{
			State: tt.state, HasMerged: tt.merged, Mergeable: tt.mergeable,
			Head: &forgejo.PRBranchInfo{RepoID: tt.headRepo}, Base: &forgejo.PRBranchInfo{RepoID: 1},
		}}
		m, err := prJSON(nil, cmdutil.Repo{}, pr, &cmdutil.JSONFlags{})
		if err != nil {
			t.Fatal(err)
		}
		if m["state"] != tt.wantState || m["mergeable"] != tt.wantMergeable ||
			m["closed"] != tt.wantClosed || m["isCrossRepository"] != tt.wantCross {
			t.Errorf("%s: state %v, mergeable %v, closed %v, isCrossRepository %v", tt.name,
				m["state"], m["mergeable"], m["closed"], m["isCrossRepository"])
		}
	}
}

func TestLatestReviews(t *testing.T) {
	a, b := &forgejo.User{ID: 1, UserName: "a"}, &forgejo.User{ID: 2, UserName: "b"}
	review := func(u *forgejo.User, s forgejo.ReviewStateType, dismissed bool) *forgejo.PullReview {
		return &forgejo.PullReview{Reviewer: u, State: s, Dismissed: dismissed}
	}
	tests := []struct {
		reviews []*forgejo.PullReview
		latest  int
		want    string
	}{
		{nil, 0, ""},
		{[]*forgejo.PullReview{review(a, forgejo.ReviewStateRequestChanges, false), review(a, forgejo.ReviewStateApproved, false)}, 1, "APPROVED"},
		{[]*forgejo.PullReview{review(a, forgejo.ReviewStateApproved, false), review(b, forgejo.ReviewStateRequestChanges, false)}, 2, "CHANGES_REQUESTED"},
		{[]*forgejo.PullReview{review(a, forgejo.ReviewStateRequestChanges, true), review(b, forgejo.ReviewStateComment, false)}, 2, ""},
		{[]*forgejo.PullReview{review(a, forgejo.ReviewStateApproved, false), review(a, forgejo.ReviewStateComment, false)}, 1, "APPROVED"},
		{[]*forgejo.PullReview{review(a, forgejo.ReviewStateRequestChanges, false), review(a, forgejo.ReviewStateComment, false)}, 1, "CHANGES_REQUESTED"},
		{[]*forgejo.PullReview{review(a, forgejo.ReviewStateApproved, false), review(a, forgejo.ReviewStateRequestChanges, true)}, 1, "APPROVED"},
		{[]*forgejo.PullReview{review(a, forgejo.ReviewStatePending, false), review(b, forgejo.ReviewStateComment, false)}, 2, ""},
	}
	for i, tt := range tests {
		latest := latestReviews(tt.reviews)
		if len(latest) != tt.latest {
			t.Errorf("case %d: %d latest reviews, want %d", i, len(latest), tt.latest)
		}
		if len(latest) > 0 && latest[len(latest)-1] != tt.reviews[len(tt.reviews)-1] {
			t.Errorf("case %d: the last review should be among the latest", i)
		}
		if got := reviewDecision(tt.reviews); got != tt.want {
			t.Errorf("case %d: reviewDecision = %q, want %q", i, got, tt.want)
		}
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct{ msg, headline, body string }{
		{"", "", ""},
		{"Fix the thing", "Fix the thing", ""},
		{"Fix the thing\n", "Fix the thing", ""},
		{"Fix the thing\n\nWhy it broke.\nHow it is fixed.\n", "Fix the thing", "Why it broke.\nHow it is fixed."},
		{"Fix the thing\r\n\r\nBody\r\n", "Fix the thing", "Body"},
	}
	for _, tt := range tests {
		h, b := splitMessage(tt.msg)
		if h != tt.headline || b != tt.body {
			t.Errorf("splitMessage(%q) = %q, %q; want %q, %q", tt.msg, h, b, tt.headline, tt.body)
		}
	}
}
