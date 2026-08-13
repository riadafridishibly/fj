package cmdutil

import (
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// refTestRepo doubles as the -R override so the factory resolves a base
// repository without reading configuration.
var refTestRepo = Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}

func testFactory() *Factory {
	return &Factory{
		RepoOverride: refTestRepo.Host + "/" + refTestRepo.FullName(),
		HostOverride: refTestRepo.Host,
	}
}

func TestPRNumberFromArg(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantRepo Repo
		want     int64
		wantErr  bool
	}{
		{"number", []string{"42"}, refTestRepo, 42, false},
		{"hash number", []string{"#42"}, refTestRepo, 42, false},
		{"repo reference", []string{"example-org/example-repo#42"}, refTestRepo, 42, false},
		{"url", []string{"https://forgejo.example.com/example-org/example-repo/pulls/42"}, refTestRepo, 42, false},
		{"repo reference disagreeing with -R", []string{"other-org/other-repo#42"}, Repo{}, 0, true},
		{"zero", []string{"0"}, Repo{}, 0, true},
		{"negative", []string{"-1"}, Repo{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, got, err := testFactory().PRNumber(tt.args)
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
			if repo != tt.wantRepo || got != tt.want {
				t.Errorf("PRNumber(%q) = %+v, %d, want %+v, %d", tt.args, repo, got, tt.wantRepo, tt.want)
			}
		})
	}
}

func TestIssueNumberFromArg(t *testing.T) {
	repo, index, err := testFactory().IssueNumber([]string{"https://forgejo.example.com/example-org/example-repo/issues/42"})
	if err != nil {
		t.Fatalf("IssueNumber: %v", err)
	}
	if repo != refTestRepo || index != 42 {
		t.Errorf("IssueNumber = %+v, %d, want %+v, 42", repo, index, refTestRepo)
	}
}

func TestIssueNumberRequiresArg(t *testing.T) {
	if _, _, err := testFactory().IssueNumber(nil); err == nil {
		t.Fatal("IssueNumber(nil) = nil error, want error")
	}
}

// Only pull requests have a branch form; an issue command must say so
// rather than looking up a branch that can never match.
func TestIssueNumberRejectsBranch(t *testing.T) {
	_, _, err := testFactory().IssueNumber([]string{"feature-1"})
	if err == nil {
		t.Fatal("IssueNumber(branch) = nil error, want error")
	}
	if !IsFlagError(err) {
		t.Errorf("IssueNumber(branch) error is not a FlagError: %v", err)
	}
	if !strings.Contains(err.Error(), "branch") {
		t.Errorf("IssueNumber(branch) error = %q, want it to mention the branch form", err)
	}
}

func TestPickBranchPR(t *testing.T) {
	repo := refTestRepo

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

// The usage specs are gh's verbatim, so a change here is a parity break.
func TestNumberResolverSpec(t *testing.T) {
	if got := IssueNumberResolver().Spec; got != "{<number> | <url>}" {
		t.Errorf("issue spec = %q, want %q", got, "{<number> | <url>}")
	}
	if got := PRNumberResolver().Spec; got != "[<number> | <url> | <branch>]" {
		t.Errorf("pr spec = %q, want %q", got, "[<number> | <url> | <branch>]")
	}
}
