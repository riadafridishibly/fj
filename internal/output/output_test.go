package output

import (
	"strings"
	"testing"
)

func lineWidth(s string) int { return displayWidth(s) }

func TestTableUnboundedNoTruncation(t *testing.T) {
	title := "a very long title that should not be truncated when width is unbounded"
	tbl := NewTable("NUM", "TITLE").Flexible(1).MaxWidth(0)
	tbl.AddRow("#1", title)

	var b strings.Builder
	tbl.Render(&b)
	if !strings.Contains(b.String(), title) {
		t.Fatalf("expected full title in unbounded output:\n%s", b.String())
	}
	if strings.Contains(b.String(), "…") {
		t.Fatalf("did not expect an ellipsis in unbounded output:\n%s", b.String())
	}
}

func TestTableTruncatesFlexibleColumnToFit(t *testing.T) {
	const maxWidth = 30
	title := "a very long title that exceeds the width budget by quite a lot"
	tbl := NewTable("NUM", "TITLE").Flexible(1).MaxWidth(maxWidth)
	tbl.AddRow("#1", title)

	var b strings.Builder
	tbl.Render(&b)

	for line := range strings.SplitSeq(strings.TrimRight(b.String(), "\n"), "\n") {
		if w := lineWidth(line); w > maxWidth {
			t.Errorf("line exceeds max width %d (got %d): %q", maxWidth, w, line)
		}
	}
	if !strings.Contains(b.String(), "…") {
		t.Errorf("expected an ellipsis marking truncation:\n%s", b.String())
	}
}

func TestTablePreservesNarrowColumns(t *testing.T) {
	// The wide flexible column (TITLE) should absorb the shrinking, not the
	// narrow ones. UPDATED must survive intact, and the flex column must be
	// padded to a consistent width so trailing columns stay aligned.
	tbl := NewTable("NUMBER", "TITLE", "UPDATED").Flexible(1).MaxWidth(40)
	tbl.AddRow("#578", "some title that is definitely too wide to fit here", "3 minutes ago")
	tbl.AddRow("#1", "short", "3 minutes ago")

	var b strings.Builder
	tbl.Render(&b)
	out := b.String()

	if !strings.Contains(out, "3 minutes ago") {
		t.Fatalf("narrow UPDATED column should be untouched:\n%s", out)
	}
	// The two data rows carry identical NUMBER-ish and UPDATED content, so once
	// the flex column is padded to a common width they must line up exactly.
	// (The header row is legitimately shorter: the last column is not right-padded.)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	data := lines[1:]
	want := lineWidth(data[0])
	for _, l := range data {
		if lineWidth(l) != want {
			t.Errorf("misaligned data rows: %q width %d, want %d", l, lineWidth(l), want)
		}
	}
}

func TestTableFloorsAtHeaderWidth(t *testing.T) {
	// Even under extreme pressure, a flexible column must not shrink below
	// its header, or the header would overflow its own column.
	tbl := NewTable("N", "DESCRIPTION").Flexible(1).MaxWidth(4)
	tbl.AddRow("1", "a description far wider than the tiny budget")

	var b strings.Builder
	tbl.Render(&b)
	if !strings.Contains(b.String(), "DESCRIPTION") {
		t.Fatalf("header should never be truncated:\n%s", b.String())
	}
}

func TestTruncateDisplay(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hell…"},
		{"héllo wörld", 6, "héllo…"},
		{"abc", 0, ""},
	}
	for _, c := range cases {
		if got := TruncateDisplay(c.in, c.width); got != c.want {
			t.Errorf("TruncateDisplay(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
		if w := displayWidth(TruncateDisplay(c.in, c.width)); c.width > 0 && w > c.width {
			t.Errorf("TruncateDisplay(%q, %d) width %d exceeds %d", c.in, c.width, w, c.width)
		}
	}
}

func TestTruncateDisplayResetsColor(t *testing.T) {
	colored := Red + "a long colored string that gets cut" + Reset
	got := TruncateDisplay(colored, 10)
	if !strings.HasSuffix(got, Reset) {
		t.Errorf("expected a trailing reset after cutting colored text: %q", got)
	}
	if displayWidth(got) > 10 {
		t.Errorf("colored truncation exceeded width: display %d", displayWidth(got))
	}
}
