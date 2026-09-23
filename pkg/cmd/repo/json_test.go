package repo

import (
	"fmt"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func TestRepoJSONMapping(t *testing.T) {
	archived := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name                       string
		repo                       apiRepository
		visibility, perm, merge    string
		archivedAt                 any
		wantMirrorURL, wantPrimary string
	}{
		{"public, no permissions", apiRepository{}, "PUBLIC", "", "", nil, "", ""},
		{"private admin", apiRepository{Repository: forgejo.Repository{
			Private: true, Internal: true, Permissions: &forgejo.Permission{Admin: true, Push: true, Pull: true},
			DefaultMergeStyle: forgejo.MergeStyleMerge,
		}}, "PRIVATE", "ADMIN", "MERGE", nil, "", ""},
		{"internal writer", apiRepository{Repository: forgejo.Repository{
			Internal: true, Permissions: &forgejo.Permission{Push: true, Pull: true}, DefaultMergeStyle: forgejo.MergeStyleRebaseMerge,
		}}, "INTERNAL", "WRITE", "REBASE_MERGE", nil, "", ""},
		{"reader", apiRepository{Repository: forgejo.Repository{
			Permissions: &forgejo.Permission{Pull: true}, DefaultMergeStyle: forgejo.MergeStyleFastForwardOnly,
		}}, "PUBLIC", "READ", "FAST_FORWARD_ONLY", nil, "", ""},
		{"unarchived keeps no archivedAt", apiRepository{ArchivedAt: time.Unix(0, 0)}, "PUBLIC", "", "", nil, "", ""},
		{"archived mirror", apiRepository{ArchivedAt: archived, Language: "Go", Repository: forgejo.Repository{
			Archived: true, Mirror: true, OriginalURL: "https://forgejo.example.com/o/r", DefaultMergeStyle: forgejo.MergeStyleSquash,
		}}, "PUBLIC", "", "SQUASH", "2026-01-02T03:04:05Z", "https://forgejo.example.com/o/r", "Go"},
		{"migrated, not a mirror", apiRepository{Repository: forgejo.Repository{OriginalURL: "https://forgejo.example.com/o/r"}},
			"PUBLIC", "", "", nil, "", ""},
	}
	for _, tt := range tests {
		m, err := repoJSON(nil, "", &tt.repo, &cmdutil.JSONFlags{})
		if err != nil {
			t.Fatal(err)
		}
		var primary string
		if p, ok := m["primaryLanguage"].(map[string]any); ok && p != nil {
			primary = p["name"].(string)
		}
		if m["visibility"] != tt.visibility || m["viewerPermission"] != tt.perm || m["viewerDefaultMergeMethod"] != tt.merge ||
			m["archivedAt"] != tt.archivedAt || m["mirrorUrl"] != tt.wantMirrorURL || primary != tt.wantPrimary {
			t.Errorf("%s: visibility %v, viewerPermission %v, viewerDefaultMergeMethod %v, archivedAt %v, mirrorUrl %v, primaryLanguage %v",
				tt.name, m["visibility"], m["viewerPermission"], m["viewerDefaultMergeMethod"], m["archivedAt"], m["mirrorUrl"], m["primaryLanguage"])
		}
	}
}

func TestLanguagesJSON(t *testing.T) {
	got := fmt.Sprint(languagesJSON(map[string]int64{"Shell": 10, "Go": 500, "Makefile": 10, "C": 10}))
	want := "[map[node:map[name:Go] size:500] map[node:map[name:C] size:10] map[node:map[name:Makefile] size:10] map[node:map[name:Shell] size:10]]"
	if got != want {
		t.Errorf("languages = %s\nwant        %s", got, want)
	}
	if got := languagesJSON(nil); got == nil || len(got) != 0 {
		t.Errorf("no languages = %v, want []", got)
	}
}
