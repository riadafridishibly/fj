package cmdutil

import (
	"strings"
	"testing"
)

func TestParseRef(t *testing.T) {
	repo := Repo{Owner: "example-org", Name: "example-repo"}
	hosted := Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}

	tests := []struct {
		name  string
		input string
		want  Ref
	}{
		{"number", "42", Ref{Number: 42}},
		{"hash number", "#42", Ref{Number: 42}},
		{"owner/repo#n", "example-org/example-repo#42", Ref{Repo: repo, Number: 42}},
		{"host/owner/repo#n", "forgejo.example.com/example-org/example-repo#42", Ref{Repo: hosted, Number: 42}},
		{"issue url", "https://forgejo.example.com/example-org/example-repo/issues/42", Ref{Repo: hosted, Number: 42}},
		{"pulls url", "https://forgejo.example.com/example-org/example-repo/pulls/42", Ref{Repo: hosted, Number: 42}},
		{"github pull spelling", "https://forgejo.example.com/example-org/example-repo/pull/42", Ref{Repo: hosted, Number: 42}},
		{"http url", "http://forgejo.example.com/example-org/example-repo/issues/42", Ref{Repo: hosted, Number: 42}},
		{"url with port", "https://forgejo.example.com:3000/example-org/example-repo/issues/42",
			Ref{Repo: Repo{Host: "forgejo.example.com:3000", Owner: "example-org", Name: "example-repo"}, Number: 42}},
		{"url with trailing slash", "https://forgejo.example.com/example-org/example-repo/issues/42/", Ref{Repo: hosted, Number: 42}},
		{"url with subpath", "https://forgejo.example.com/example-org/example-repo/issues/42/comments", Ref{Repo: hosted, Number: 42}},
		{"url with fragment", "https://forgejo.example.com/example-org/example-repo/issues/42#issuecomment-7", Ref{Repo: hosted, Number: 42}},
		{"url with query", "https://forgejo.example.com/example-org/example-repo/issues/42?tab=files", Ref{Repo: hosted, Number: 42}},
		{"branch", "feature-1", Ref{Branch: "feature-1"}},
		{"branch with slash", "riad/feature-1", Ref{Branch: "riad/feature-1"}},
		{"branch starting with a digit", "42x", Ref{Branch: "42x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRef(tt.input, IssueRefKind)
			if err != nil {
				t.Fatalf("ParseRef(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseRef(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRefErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"zero", "0"},
		{"negative", "-1"},
		{"hash zero", "#0"},
		{"hash not a number", "#main"},
		{"repo hash zero", "example-org/example-repo#0"},
		{"repo hash not a number", "example-org/example-repo#main"},
		{"bare name with hash", "example-repo#42"},
		{"too many segments with hash", "a/example-org/example-repo/extra#42"},
		{"url without a number", "https://forgejo.example.com/example-org/example-repo/issues"},
		{"url with a bad number", "https://forgejo.example.com/example-org/example-repo/issues/none"},
		{"url with an unknown route", "https://forgejo.example.com/example-org/example-repo/releases/42"},
		{"url without a repository", "https://example.com/not-an-issue"},
		{"url without a host", "https:///example-org/example-repo/issues/42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, kind := range []RefKind{IssueRefKind, PRRefKind} {
				got, err := ParseRef(tt.input, kind)
				if err == nil {
					t.Fatalf("ParseRef(%q, %q) = %+v, want error", tt.input, kind, got)
				}
				if !IsFlagError(err) {
					t.Errorf("ParseRef(%q, %q) error is not a FlagError: %v", tt.input, kind, err)
				}
			}
		})
	}
}

// The example URL in a parse error names the route of the command the
// user ran, not always the issue one.
func TestParseRefErrorNamesTheKindsRoute(t *testing.T) {
	for kind, want := range map[RefKind]string{
		IssueRefKind: "https://HOST/OWNER/REPO/issues/42",
		PRRefKind:    "https://HOST/OWNER/REPO/pulls/42",
	} {
		_, err := ParseRef("example-org/example-repo#0", kind)
		if err == nil {
			t.Fatalf("ParseRef(%q) = nil error, want error", kind)
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ParseRef error for %q = %q, want it to contain %q", kind, err, want)
		}
	}
}
