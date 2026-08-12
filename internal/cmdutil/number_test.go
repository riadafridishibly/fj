package cmdutil

import (
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestPRNumberFromArg(t *testing.T) {
	f := &Factory{}
	repo := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}

	tests := []struct {
		name    string
		args    []string
		want    int64
		wantErr bool
	}{
		{"number", []string{"42"}, 42, false},
		{"not a number", []string{"main"}, 0, true},
		{"zero", []string{"0"}, 0, true},
		{"negative", []string{"-1"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.PRNumber(repo, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PRNumber(%q) = %d, want error", tt.args, got)
				}
				if !IsFlagError(err) {
					t.Errorf("PRNumber(%q) error is not a FlagError: %v", tt.args, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("PRNumber(%q): %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("PRNumber(%q) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestIssueNumberRequiresArg(t *testing.T) {
	f := &Factory{}
	if _, err := f.IssueNumber(Repo{}, nil); err == nil {
		t.Fatal("IssueNumber(nil) = nil error, want error")
	}
}

func TestPickBranchPR(t *testing.T) {
	repo := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}

	pr := func(index int64, ref, headRepo string) *forgejo.PullRequest {
		head := &forgejo.PRBranchInfo{Ref: ref}
		if headRepo != "" {
			head.Repository = &forgejo.Repository{FullName: headRepo}
		}
		return &forgejo.PullRequest{Index: index, Head: head}
	}

	tests := []struct {
		name    string
		prs     []*forgejo.PullRequest
		want    int64
		wantErr string
	}{
		{
			name: "single match",
			prs:  []*forgejo.PullRequest{pr(1, "other", ""), pr(7, "feature", "")},
			want: 7,
		},
		{
			name: "head repo unknown counts as local",
			prs:  []*forgejo.PullRequest{pr(7, "feature", "")},
			want: 7,
		},
		{
			name: "base repo wins over fork",
			prs:  []*forgejo.PullRequest{pr(3, "feature", "someone/fork"), pr(7, "feature", "example-org/example-repo")},
			want: 7,
		},
		{
			name: "fork used when nothing local matches",
			prs:  []*forgejo.PullRequest{pr(3, "feature", "someone/fork")},
			want: 3,
		},
		{
			name:    "no match",
			prs:     []*forgejo.PullRequest{pr(1, "other", "")},
			wantErr: "no open pull request found",
		},
		{
			name:    "ambiguous",
			prs:     []*forgejo.PullRequest{pr(3, "feature", ""), pr(7, "feature", "")},
			wantErr: "#3, #7",
		},
		{
			name:    "missing head",
			prs:     []*forgejo.PullRequest{{Index: 1}},
			wantErr: "no open pull request found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pickBranchPR(tt.prs, repo, "feature")
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("pickBranchPR = %d, want error containing %q", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("pickBranchPR error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("pickBranchPR: %v", err)
			}
			if got != tt.want {
				t.Errorf("pickBranchPR = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNumberResolverArgSpec(t *testing.T) {
	if got := IssueNumberResolver().ArgSpec("issue"); got != "<issue>" {
		t.Errorf("issue ArgSpec = %q, want %q", got, "<issue>")
	}
	if got := PRNumberResolver().ArgSpec("pr"); got != "[<pr>]" {
		t.Errorf("pr ArgSpec = %q, want %q", got, "[<pr>]")
	}
}
