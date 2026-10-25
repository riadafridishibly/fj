package pr

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

func TestMergeMatchHeadCommit(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	tests := []struct {
		name    string
		args    []string
		want    string
		status  int
		message string
	}{
		{"matching head", []string{"--match-head-commit", sha}, sha, http.StatusOK, ""},
		{"empty flag", []string{"--match-head-commit", ""}, "", http.StatusOK, ""},
		{"omitted flag", nil, "", http.StatusOK, ""},
		{"stale head", []string{"--match-head-commit", sha}, sha, http.StatusConflict, "head out of date"},
		{"JSON stale head", []string{"--match-head-commit", sha, "--json", "state"}, sha, http.StatusConflict, "head out of date"},
		{"unconditional refused", nil, "", http.StatusForbidden, "User not allowed to merge PR"},
		{"missing repository", []string{"--match-head-commit", sha}, sha, http.StatusNotFound, "The target couldn't be found."},
		{"auto refused", []string{"--auto", "--match-head-commit", sha}, sha, http.StatusConflict, "head out of date"},
		{"auto scheduled", []string{"--auto", "--match-head-commit", sha}, sha, http.StatusCreated, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got forgejo.MergePullRequestOption
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v1/version" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"version":"14.0.0"}`))
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/repos/owner/repo/pulls/42/merge" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				requests++
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("reading merge request: %v", err)
				}
				t.Logf("merge request: %s %s body=%s", r.Method, r.URL.Path, body)
				// Assert the wire keys independently of the SDK's JSON tags.
				var wire struct {
					Do           string `json:"Do"`
					HeadCommitID string `json:"head_commit_id"`
				}
				if err := json.Unmarshal(body, &wire); err != nil || wire.Do != "merge" || wire.HeadCommitID != tt.want {
					t.Errorf("request body = %s (%v), want Do=merge and head_commit_id=%q", body, err, tt.want)
				}
				if err := json.Unmarshal(body, &got); err != nil {
					t.Errorf("decoding merge options: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				if tt.message != "" {
					_ = json.NewEncoder(w).Encode(map[string]string{"message": tt.message})
				}
			}))
			defer server.Close()

			host := strings.TrimPrefix(server.URL, "http://")
			t.Setenv("FJ_INSECURE", "1")
			t.Setenv("FJ_HOST", host)
			t.Setenv("FJ_TOKEN", "test-token")
			f := &cmdutil.Factory{
				RepoOverride: "owner/repo",
				Config: func() (*config.Config, error) {
					return &config.Config{}, nil
				},
			}
			cmd := NewCmdMerge(f)
			cmd.SetArgs(append([]string{"42"}, tt.args...))
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			err := cmd.Execute()
			if tt.message != "" {
				if err == nil || !strings.Contains(err.Error(), tt.message) {
					t.Errorf("error = %v, want server message %q", err, tt.message)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if requests != 1 {
				t.Fatalf("merge requests = %d, want 1", requests)
			}
			if got.HeadCommitId != tt.want {
				t.Errorf("HeadCommitId = %q, want %q", got.HeadCommitId, tt.want)
			}
			if got.Style != forgejo.MergeStyleMerge {
				t.Errorf("Style = %q, want merge", got.Style)
			}
		})
	}
}
