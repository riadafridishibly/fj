package review

import (
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestGhState(t *testing.T) {
	tests := []struct {
		state     forgejo.ReviewStateType
		dismissed bool
		want      string
	}{
		{forgejo.ReviewStateApproved, false, "APPROVED"},
		{forgejo.ReviewStateRequestChanges, false, "CHANGES_REQUESTED"},
		{forgejo.ReviewStateComment, false, "COMMENTED"},
		{forgejo.ReviewStatePending, false, "PENDING"},
		{forgejo.ReviewStateApproved, true, "DISMISSED"},
		{forgejo.ReviewStateRequestReview, false, "REQUEST_REVIEW"},
	}
	for _, tt := range tests {
		if got := ghState(&forgejo.PullReview{State: tt.state, Dismissed: tt.dismissed}); got != tt.want {
			t.Errorf("ghState(%s, dismissed=%v) = %s, want %s", tt.state, tt.dismissed, got, tt.want)
		}
	}
}
