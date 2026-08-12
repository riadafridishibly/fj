package cmdutil

import "testing"

func TestRepoFromFullName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Repo
		wantErr bool
	}{
		{"owner/repo", "example-org/example-repo", Repo{Owner: "example-org", Name: "example-repo"}, false},
		{"host/owner/repo", "forgejo.example.com/example-org/example-repo", Repo{Host: "forgejo.example.com", Owner: "example-org", Name: "example-repo"}, false},
		{"bare name", "example-repo", Repo{}, true},
		{"four segments", "forgejo.example.com/a/b/c", Repo{}, true},
		{"empty segment", "example-org//example-repo", Repo{}, true},
		{"trailing slash", "example-org/example-repo/", Repo{}, true},
		{"leading slash", "/example-org/example-repo", Repo{}, true},
		{"empty", "", Repo{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RepoFromFullName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RepoFromFullName(%q) = %+v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RepoFromFullName(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("RepoFromFullName(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}
