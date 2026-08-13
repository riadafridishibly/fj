package issue

import (
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestIssueLines(t *testing.T) {
	issue := func(index int64, title string) *forgejo.Issue {
		return &forgejo.Issue{Index: index, Title: title}
	}

	tests := []struct {
		name    string
		section issueSection
		want    []string
	}{
		{
			name:    "empty section has no lines so the empty state prints",
			section: issueSection{},
		},
		{
			name:    "one line per issue",
			section: issueSection{Issues: []*forgejo.Issue{issue(1, "First"), issue(2, "Second")}, Total: 2},
			want:    []string{"#1", "#2"},
		},
		{
			// The listing is capped at statusLimit but the server counts
			// everything, so the tail is the difference.
			name:    "tail counts what the page left out",
			section: issueSection{Issues: []*forgejo.Issue{issue(1, "First")}, Total: 4},
			want:    []string{"#1", "And 3 more"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := issueLines(tt.section)
			if len(got) != len(tt.want) {
				t.Fatalf("issueLines = %q, want %d lines", got, len(tt.want))
			}
			for i, want := range tt.want {
				if !strings.Contains(got[i], want) {
					t.Errorf("line %d = %q, want it to contain %q", i, got[i], want)
				}
			}
		})
	}
}
