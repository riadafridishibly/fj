package output

import (
	"strings"
	"testing"
)

// pipeOutput makes the test see stdout as a pipe even when go test streams
// to a terminal.
func pipeOutput(t *testing.T) {
	saved := isTTY
	isTTY = false
	t.Cleanup(func() { isTTY = saved })
}

func TestTemplateFuncs(t *testing.T) {
	pipeOutput(t)
	const input = `{"title":"Fix it","id":12345678901,"t":"2026-01-02T03:04:05Z",
		"labels":[{"name":"bug"},{"name":"docs"}],"rows":[{"a":"x","b":"1"},{"a":"longer","b":"2"}]}`
	tests := []struct{ tmpl, want string }{
		{`{{.title}}`, "Fix it"},
		{`{{.id}}`, "12345678901"},
		{`{{join ", " (pluck "name" .labels)}}`, "bug, docs"},
		{`{{range .rows}}{{tablerow .a .b}}{{end}}`, "x       1\nlonger  2\n"},
		{`{{tablerow "A" "B"}}{{tablerender}}end`, "A  B\nend"},
		{`{{timefmt "2006-01-02" .t}}`, "2026-01-02"},
		{`{{truncate 4 .title}}`, "Fix…"},
		{`{{color "green" .title}}`, Green + "Fix it" + Reset},
		{`{{autocolor "green" .title}}`, "Fix it"},
		{`{{hyperlink "https://forgejo.example.com" .title}}`, "Fix it"},
		{`{{contains "ix" .title}} {{hasPrefix "Fix" .title}} {{hasSuffix "x" .title}} {{regexMatch "^F.x" .title}}`, "true true false true"},
	}
	for _, tt := range tests {
		tmpl, err := ParseTemplate(tt.tmpl)
		if err != nil {
			t.Fatalf("%s: %v", tt.tmpl, err)
		}
		var b strings.Builder
		if err := tmpl.Execute(&b, []byte(input)); err != nil {
			t.Fatalf("%s: %v", tt.tmpl, err)
		}
		if b.String() != tt.want {
			t.Errorf("%s = %q, want %q", tt.tmpl, b.String(), tt.want)
		}
	}
}

func TestExportJSON(t *testing.T) {
	pipeOutput(t)
	v := []map[string]any{{"title": "a", "labels": []string{}, "number": 1}}

	var b strings.Builder
	if err := ExportJSON(&b, v, nil, nil); err != nil {
		t.Fatal(err)
	}
	if want := `[{"labels":[],"number":1,"title":"a"}]` + "\n"; b.String() != want {
		t.Errorf("piped = %q, want %q", b.String(), want)
	}

	isTTY = true
	b.Reset()
	if err := ExportJSON(&b, map[string]any{"a": 1}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"a\": 1\n}\n"; b.String() != want {
		t.Errorf("terminal = %q, want %q", b.String(), want)
	}

	code, err := ParseJQ(".[].title")
	if err != nil {
		t.Fatal(err)
	}
	b.Reset()
	if err := ExportJSON(&b, v, code, nil); err != nil {
		t.Fatal(err)
	}
	if b.String() != "a\n" {
		t.Errorf("jq = %q, want %q", b.String(), "a\n")
	}
}
