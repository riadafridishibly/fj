//go:build integration

package integration

import (
	"encoding/json"
	"errors"
	"maps"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// TestIssueListJSONFields checks that --json keeps only the named fields and
// prints them compactly on a pipe, as gh does.
func TestIssueListJSONFields(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "-L", "100", "--json", "number,title")

	if strings.Count(strings.TrimSpace(out), "\n") != 0 {
		t.Errorf("piped output should be one line:\n%s", out)
	}
	var issues []map[string]any
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	var found bool
	for _, iss := range issues {
		if len(iss) != 2 || iss["number"] == nil || iss["title"] == nil {
			t.Errorf("want exactly number and title, got %v", iss)
		}
		found = found || iss["title"] == "First issue"
	}
	if !found {
		t.Errorf("First issue missing from:\n%s", out)
	}
}

// TestIssueJSONGhShape reads an issue through create, view and edit and
// checks the names and shapes against gh's.
func TestIssueJSONGhShape(t *testing.T) {
	repo := adminUser + "/test-repo"
	out := mustRunFJ(t, "issue", "create", "-R", repo, "--title", "gh shape target",
		"--label", "bug", "--milestone", "v1.0", "--json", "number,isPinned,comments")
	var created struct {
		Number   int64 `json:"number"`
		IsPinned *bool `json:"isPinned"`
		Comments []any `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if created.Number == 0 || created.IsPinned == nil || *created.IsPinned || created.Comments == nil {
		t.Errorf("create: want a number, isPinned false and comments [], got %s", out)
	}
	num := strconv.FormatInt(created.Number, 10)

	if _, _, err := testClient.CreateIssueComment(adminUser, "test-repo", created.Number,
		forgejo.CreateIssueCommentOption{Body: "shape comment"}); err != nil {
		t.Fatalf("commenting: %v", err)
	}
	mustRunFJ(t, "api", "-X", "POST", "-R", repo, "repos/{owner}/{repo}/issues/"+num+"/pin")

	out = mustRunFJ(t, "issue", "view", num, "-R", repo, "--json",
		"assignees,author,closedAt,comments,isPinned,labels,milestone,state,url")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	keys := slices.Sorted(maps.Keys(got))
	if strings.Join(keys, ",") != "assignees,author,closedAt,comments,isPinned,labels,milestone,state,url" {
		t.Errorf("keys = %v", keys)
	}

	author, _ := got["author"].(map[string]any)
	if author["login"] != adminUser || author["is_bot"] != false || author["id"] == nil || author["name"] == nil {
		t.Errorf("author = %v, want gh's {id, is_bot, login, name}", author)
	}
	labels, _ := got["labels"].([]any)
	if len(labels) != 1 {
		t.Fatalf("labels = %v, want one", got["labels"])
	}
	if l := labels[0].(map[string]any); l["name"] != "bug" || l["color"] != "ee0701" || l["id"] == nil {
		t.Errorf("label = %v, want name bug and color ee0701", l)
	}
	if ms, _ := got["milestone"].(map[string]any); ms["title"] != "v1.0" || ms["number"] == nil {
		t.Errorf("milestone = %v", got["milestone"])
	}
	if got["state"] != "OPEN" || got["closedAt"] != nil || got["isPinned"] != true {
		t.Errorf("state %v, closedAt %v, isPinned %v; want OPEN, null, true", got["state"], got["closedAt"], got["isPinned"])
	}
	if url, _ := got["url"].(string); !strings.HasSuffix(url, "/"+repo+"/issues/"+num) {
		t.Errorf("url = %q", url)
	}
	if a, ok := got["assignees"].([]any); !ok || len(a) != 0 {
		t.Errorf("assignees = %v, want []", got["assignees"])
	}
	comments, _ := got["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("comments = %v, want one", got["comments"])
	}
	c := comments[0].(map[string]any)
	if url, _ := c["url"].(string); c["body"] != "shape comment" || url == "" || c["id"] == nil ||
		c["createdAt"] == nil || c["author"].(map[string]any)["login"] != adminUser {
		t.Errorf("comment = %v, want gh's {author, body, createdAt, id, url}", c)
	}

	out = mustRunFJ(t, "issue", "edit", num, "-R", repo, "--add-label", "documentation",
		"--json", "labels", "--jq", "[.labels[].name] | sort | join(\",\")")
	if strings.TrimSpace(out) != "bug,documentation" {
		t.Errorf("edit --jq = %q, want bug,documentation", out)
	}
}

// TestIssueJSONFlagErrors pins gh's messages and exit code 1 for misuse of
// the formatting flags.
func TestIssueJSONFlagErrors(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"--json"}, []string{"Specify one or more comma-separated fields for `--json`:", "\n  number\n"}},
		{[]string{"--json", "bogus"}, []string{`Unknown JSON field: "bogus"`, "Available fields:", "\n  title\n"}},
		{[]string{"--jq", ".[].title"}, []string{"cannot use `--jq` without specifying `--json`"}},
		{[]string{"--template", "{{.}}"}, []string{"cannot use `--template` without specifying `--json`"}},
	}
	for _, tt := range tests {
		args := append([]string{"issue", "list", "-R", adminUser + "/test-repo"}, tt.args...)
		stdout, stderr, err := runFJ(args...)
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Errorf("%v: err = %v, want exit 1", tt.args, err)
		}
		if stdout != "" {
			t.Errorf("%v: stdout = %q, want nothing", tt.args, stdout)
		}
		for _, w := range tt.want {
			if !strings.Contains(stderr, w) {
				t.Errorf("%v: stderr missing %q:\n%s", tt.args, w, stderr)
			}
		}
	}
}

func TestIssueListJQ(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "-L", "100", "--json", "title", "--jq", ".[].title")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if !slices.Contains(lines, "First issue") || !slices.Contains(lines, "Second issue") {
		t.Errorf("want one raw title per line, got:\n%s", out)
	}
}

func TestIssueListTemplate(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/test-repo", "-L", "100", "--json", "number,title",
		"--template", `{{range .}}{{if eq .title "First issue"}}{{tablerow (printf "#%v" .number) .title}}{{end}}{{end}}`)
	if !strings.Contains(out, "#1  First issue\n") {
		t.Errorf("template output missing the #1 row:\n%s", out)
	}
}

func TestIssueListJSONEmpty(t *testing.T) {
	out := mustRunFJ(t, "issue", "list", "-R", adminUser+"/another-repo", "--state", "all", "--json", "number")
	if out != "[]\n" {
		t.Errorf("empty list = %q, want []", out)
	}
}
