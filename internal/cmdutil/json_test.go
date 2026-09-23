package cmdutil

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"
)

// runJSONCmd runs a command carrying the JSON flags and a --web flag, and
// returns what reached RunE.
func runJSONCmd(t *testing.T, args ...string) (*JSONFlags, bool, error) {
	t.Helper()
	var j JSONFlags
	var web, ran bool
	cmd := &cobra.Command{
		Use:           "list",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          func(*cobra.Command, []string) error { ran = true; return nil },
	}
	cmd.Flags().BoolVar(&web, "web", false, "")
	AddJSONFlags(cmd, &j, []string{"number", "title"}, []string{"timeline"}, true)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	err := cmd.Execute()
	return &j, ran, err
}

func TestJSONFlagErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--json"}, "Specify one or more comma-separated fields for `--json`:\n  number\n  timeline\n  title"},
		{[]string{"--json", "bogus"}, "Unknown JSON field: \"bogus\"\nAvailable fields:\n  number\n  timeline\n  title"},
		{[]string{"--jq", "."}, "cannot use `--jq` without specifying `--json`"},
		{[]string{"--template", "{{.}}"}, "cannot use `--template` without specifying `--json`"},
		{[]string{"--json", "title", "--web"}, "cannot use `--web` with `--json`"},
		{[]string{"--json", "title", "--jq", ".["}, "failed to parse jq expression"},
		{[]string{"--json", "title", "-t", "{{"}, "failed to parse template"},
	}
	for _, tt := range tests {
		_, ran, err := runJSONCmd(t, tt.args...)
		if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
			t.Errorf("%v: err = %v, want %q", tt.args, err, tt.want)
		}
		if ran {
			t.Errorf("%v: RunE ran despite the error", tt.args)
		}
		if IsFlagError(err) {
			t.Errorf("%v: got a FlagError, which exits 2; gh exits 1", tt.args)
		}
	}
}

func TestJSONFlagsWrite(t *testing.T) {
	j, _, err := runJSONCmd(t, "--json", "title,number,title", "--jq", ".[].title", "--template", "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(j.Fields, ","); got != "number,title" {
		t.Errorf("Fields = %s, want number,title", got)
	}

	var b strings.Builder
	data := []map[string]any{{"number": 1, "title": "a", "body": "dropped"}}
	if err := j.Write(&b, data); err != nil {
		t.Fatal(err)
	}
	if b.String() != "a\n" {
		t.Errorf("--jq should win over --template: got %q", b.String())
	}

	// --jq . prints compact JSON on a terminal too, so the check holds
	// wherever the test runs.
	j, _, _ = runJSONCmd(t, "--json", "number,timeline", "--jq", ".")
	b.Reset()
	if err := j.Write(&b, data); err != nil {
		t.Fatal(err)
	}
	if want := `[{"number":1}]` + "\n"; b.String() != want {
		t.Errorf("projection = %q, want %q", b.String(), want)
	}
	b.Reset()
	if err := j.Write(&b, []map[string]any(nil)); err != nil {
		t.Fatal(err)
	}
	if b.String() != "[]\n" {
		t.Errorf("empty list = %q, want []", b.String())
	}
}

func TestJSONFieldsHelp(t *testing.T) {
	got := fieldsHelp([]string{"number", "title"}, []string{"timeline"})
	want := "\nJSON FIELDS\n  Names and shapes follow gh (GitHub CLI); fields Forgejo lacks are omitted.\n" +
		"  number, title\n  fj-only: timeline\n"
	if got != want {
		t.Errorf("fieldsHelp = %q, want %q", got, want)
	}

	got = fieldsHelp(nil, []string{"path", "reviewId"})
	want = "\nJSON FIELDS\n  gh (GitHub CLI) has no such command; the names follow gh's style.\n" +
		"  path, reviewId\n"
	if got != want {
		t.Errorf("fj-only fieldsHelp = %q, want %q", got, want)
	}
}

// TestJSONFlagsLong: without the shorthands, -q and -t do not parse, as on
// gh auth status.
func TestJSONFlagsLong(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "hosts", "--jq", ".hosts"},
		{"--json", "hosts", "--template", "{{.hosts}}"},
		{"--json", "hosts", "-q", ".hosts"},
		{"--json", "hosts", "-t", "{{.hosts}}"},
	} {
		var j JSONFlags
		cmd := &cobra.Command{Use: "status", SilenceErrors: true, SilenceUsage: true, RunE: func(*cobra.Command, []string) error { return nil }}
		AddJSONFlagsLong(cmd, &j, []string{"hosts"}, nil)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if short := strings.HasPrefix(args[2], "-") && !strings.HasPrefix(args[2], "--"); short != (err != nil) {
			t.Errorf("%v: err = %v", args, err)
		}
	}
}

// TestJSONShapes pins the shared shapes the fj-only commands print.
func TestJSONShapes(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("", 3600))
	due := time.Date(2031, 1, 31, 0, 0, 0, 0, time.UTC)
	never := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	zero := time.Time{}
	tests := []struct {
		name string
		got  any
		want string
	}{
		{"comment", JSONComment(&forgejo.Comment{ID: 7, Body: "hi", HTMLURL: "u", Created: created, Updated: created,
			Poster: &forgejo.User{UserName: "alice"}}),
			`{"author":{"login":"alice"},"body":"hi","createdAt":"2026-01-02T02:04:05Z","id":7,"includesCreatedEdit":false,"updatedAt":"2026-01-02T02:04:05Z","url":"u"}`},
		{"edited comment without poster", JSONComment(&forgejo.Comment{Created: created, Updated: created.Add(time.Minute)}),
			`{"author":{"login":""},"body":"","createdAt":"2026-01-02T02:04:05Z","id":0,"includesCreatedEdit":true,"updatedAt":"2026-01-02T02:05:05Z","url":""}`},
		{"asset", JSONAsset(&forgejo.Attachment{ID: 3, Name: "a.txt", Size: 10, DownloadCount: 2, Created: created, DownloadURL: "d"}),
			`{"createdAt":"2026-01-02T02:04:05Z","downloadCount":2,"id":3,"name":"a.txt","size":10,"url":"d"}`},
		{"milestone", JSONMilestone(&forgejo.Milestone{ID: 1, Title: "v1", Deadline: &due}),
			`{"description":"","dueOn":"2031-01-31T00:00:00Z","number":1,"title":"v1"}`},
		{"milestone due 9999", JSONMilestone(&forgejo.Milestone{Deadline: &never})["dueOn"], `null`},
		{"milestone zero due", JSONMilestone(&forgejo.Milestone{Deadline: &zero})["dueOn"], `null`},
		{"milestone no due", JSONMilestone(&forgejo.Milestone{})["dueOn"], `null`},
	}
	for _, tt := range tests {
		b, err := json.Marshal(tt.got)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, b, tt.want)
		}
	}
}
