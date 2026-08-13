package cmdutil

import (
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"
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
			got, gotRepo, err := f.PRNumber(repo, tt.args)
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
			if gotRepo != repo {
				t.Errorf("PRNumber(%q) repo = %v, want %v", tt.args, gotRepo, repo)
			}
		})
	}
}

func TestParseNumber(t *testing.T) {
	if got, err := ParseNumber("12345", "review id"); err != nil || got != 12345 {
		t.Fatalf("ParseNumber(12345) = (%d, %v), want (12345, nil)", got, err)
	}

	for _, arg := range []string{"0", "-1", "1.5", "", "abc"} {
		_, err := ParseNumber(arg, "review id")
		if err == nil {
			t.Errorf("ParseNumber(%q) = nil error, want a usage error", arg)
			continue
		}
		if !IsFlagError(err) {
			t.Errorf("ParseNumber(%q) error is not a FlagError: %v", arg, err)
		}
		if want := "invalid review id: " + arg; err.Error() != want {
			t.Errorf("ParseNumber(%q) error = %q, want %q", arg, err, want)
		}
	}
}

func TestIssueNumberRequiresArg(t *testing.T) {
	f := &Factory{}
	if _, _, err := f.IssueNumber(Repo{}, nil); err == nil {
		t.Fatal("IssueNumber(nil) = nil error, want error")
	}
}

func TestRequiredPRNumberRequiresArg(t *testing.T) {
	f := &Factory{}
	_, _, err := f.RequiredPRNumber(Repo{}, nil)
	if err == nil {
		t.Fatal("RequiredPRNumber(nil) = nil error, want error")
	}
	if !IsFlagError(err) {
		t.Errorf("RequiredPRNumber(nil) error is not a FlagError: %v", err)
	}
}

func TestBranchMatchesPick(t *testing.T) {
	repo := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}

	pr := func(index int64, ref, headRepo string) *forgejo.PullRequest {
		head := &forgejo.PRBranchInfo{Ref: ref}
		if headRepo != "" {
			head.Repository = &forgejo.Repository{FullName: headRepo}
		}
		return &forgejo.PullRequest{Index: index, Head: head}
	}

	tests := []struct {
		name      string
		prs       []*forgejo.PullRequest
		want      int64
		wantFound bool
		wantErr   string
	}{
		{
			name:      "single match",
			prs:       []*forgejo.PullRequest{pr(1, "other", "example-org/example-repo"), pr(7, "feature", "example-org/example-repo")},
			want:      7,
			wantFound: true,
		},
		{
			name:      "deleted fork head counts as foreign and loses to local",
			prs:       []*forgejo.PullRequest{pr(3, "feature", ""), pr(7, "feature", "example-org/example-repo")},
			want:      7,
			wantFound: true,
		},
		{
			name:      "deleted fork head used when nothing local matches",
			prs:       []*forgejo.PullRequest{pr(3, "feature", "")},
			want:      3,
			wantFound: true,
		},
		{
			name:      "base repo wins over fork",
			prs:       []*forgejo.PullRequest{pr(3, "feature", "someone/fork"), pr(7, "feature", "example-org/example-repo")},
			want:      7,
			wantFound: true,
		},
		{
			name:      "fork used when nothing local matches",
			prs:       []*forgejo.PullRequest{pr(3, "feature", "someone/fork")},
			want:      3,
			wantFound: true,
		},
		{
			name: "no match",
			prs:  []*forgejo.PullRequest{pr(1, "other", "")},
		},
		{
			name:    "ambiguous",
			prs:     []*forgejo.PullRequest{pr(3, "feature", "example-org/example-repo"), pr(7, "feature", "example-org/example-repo")},
			wantErr: "#3, #7",
		},
		{
			name:    "ambiguous among foreign matches",
			prs:     []*forgejo.PullRequest{pr(3, "feature", ""), pr(7, "feature", "someone/fork")},
			wantErr: "#3, #7",
		},
		{
			name: "missing head",
			prs:  []*forgejo.PullRequest{{Index: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m branchMatches
			m.collect(tt.prs, repo, "feature")
			got, found, err := m.pick("feature")
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("pick = %d, want error containing %q", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("pick error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("pick: %v", err)
			}
			if found != tt.wantFound {
				t.Fatalf("pick found = %v, want %v", found, tt.wantFound)
			}
			if got != tt.want {
				t.Errorf("pick = %d, want %d", got, tt.want)
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

func TestNumberResolverArgsDerivedFromOptional(t *testing.T) {
	cmd := &cobra.Command{}
	if err := IssueNumberResolver().Args()(cmd, nil); err == nil {
		t.Error("required resolver accepted zero args")
	}
	if err := PRNumberResolver().Args()(cmd, nil); err != nil {
		t.Errorf("optional resolver rejected zero args: %v", err)
	}
	if err := PRNumberResolver().Args()(cmd, []string{"1", "2"}); err == nil {
		t.Error("optional resolver accepted two args")
	}
}

func TestGroupDispatchArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "comment"}

	if err := GroupDispatchArgs(cmd, nil); err != nil {
		t.Errorf("no args: %v", err)
	}
	if err := GroupDispatchArgs(cmd, []string{"42"}); err != nil {
		t.Errorf("numeric arg: %v", err)
	}
	err := GroupDispatchArgs(cmd, []string{"lst"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "lst"`) {
		t.Errorf("mistyped subcommand error = %v, want unknown command", err)
	}
	if err := GroupDispatchArgs(cmd, []string{"1", "2"}); err == nil {
		t.Error("two args accepted")
	}
}
