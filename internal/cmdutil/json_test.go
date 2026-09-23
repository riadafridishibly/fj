package cmdutil

import (
	"io"
	"strings"
	"testing"

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
