package cmdutil

import (
	"slices"
	"testing"
)

func TestPagingDone(t *testing.T) {
	tests := []struct {
		name                  string
		pageLen, sofar, total int
		want                  bool
	}{
		{"empty page", 0, 30, 0, true},
		{"short page with more to come", 30, 30, 45, false},
		{"short page, no total header", 30, 30, 0, false},
		{"total reached", 15, 45, 45, true},
	}
	for _, tt := range tests {
		if got := PagingDone(tt.pageLen, tt.sofar, tt.total); got != tt.want {
			t.Errorf("%s: PagingDone(%d, %d, %d) = %v, want %v", tt.name, tt.pageLen, tt.sofar, tt.total, got, tt.want)
		}
	}
}

func TestLabelSetIDs(t *testing.T) {
	set := LabelSet{"bug": 1, "tech-debt": 2}

	got, err := set.IDs([]string{"Tech-Debt", "bug"})
	if err != nil || !slices.Equal(got, []int64{2, 1}) {
		t.Errorf("IDs = %v, %v, want [2 1]", got, err)
	}
	if got, err := set.IDs(nil); err != nil || got != nil {
		t.Errorf("IDs(nil) = %v, %v, want nil", got, err)
	}
	if _, err := set.IDs([]string{"bug", "nope"}); err == nil || err.Error() != "'nope' not found" {
		t.Errorf("IDs with an unknown name: err = %v, want 'nope' not found", err)
	}
}
