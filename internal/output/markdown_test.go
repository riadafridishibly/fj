package output

import (
	"strings"
	"testing"
)

const personaTable = `| Persona | Ladder says | Confidence | Coverage |
|---|---|---|---|
| A. Empty persona, no signals | L1 | **50.0** | 0 |
| B. New legit user: one Google, email verified | L2 | 85.8 → **71.1** | 22 |
| C. Established dev: GitHub 9y + MFA + active, Google, LinkedIn, names agree | L4 | 98.0 → **92.0** | 66 |
| F. Power user on VPN: Apple (real_user) + MS work + GitHub 6y, datacenter ASN flag | L4 | 98.0 → **91.0** | 66 |
`

const twoColumnTable = `| Key | Value |
|---|---|
| a very long key column that eats most of the available terminal width here | short |
| k2 | another quite long value that also wants a good chunk of the width budget |
`

// renderedCells mirrors what extractTables feeds to renderTable: cells are
// markdown-rendered (so they carry ANSI) before any width math happens.
func renderedCells(md string) ([]string, [][]string) {
	lines := strings.Split(strings.TrimSpace(md), "\n")
	headers := parseTableRow(lines[0])
	rows := make([][]string, 0, len(lines)-2)
	for _, ln := range lines[2:] {
		rows = append(rows, parseTableRow(ln))
	}
	linkIdx, _ := collectTableLinks(headers, rows)
	for j := range headers {
		headers[j] = renderCellMarkdown(headers[j], linkIdx)
	}
	for _, row := range rows {
		for j := range row {
			row[j] = renderCellMarkdown(row[j], linkIdx)
		}
	}
	return headers, rows
}

// rightEdgeIntact reports whether every line still ends in a border glyph. A
// table that overflowed its width gets hard-cropped, losing the right edge.
func rightEdgeIntact(table string) bool {
	for line := range strings.SplitSeq(strings.TrimSpace(table), "\n") {
		plain := strings.TrimRight(stripEscapes(line), " ")
		if plain == "" {
			continue
		}
		r := []rune(plain)
		switch r[len(r)-1] {
		case '│', '┐', '┘', '┤':
		default:
			return false
		}
	}
	return true
}

func stripEscapes(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\033':
			inEscape = true
		case inEscape && (r == 'm' || r == '\\'):
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func tableWidth(table string) int {
	w := 0
	for line := range strings.SplitSeq(table, "\n") {
		if lw := DisplayWidth(line); lw > w {
			w = lw
		}
	}
	return w
}

// A table whose overflow is smaller than its border budget used to skip
// lipgloss's shrink path entirely and get cropped instead of wrapped.
func TestRenderTableWrapsInsteadOfCropping(t *testing.T) {
	for name, md := range map[string]string{"persona": personaTable, "twocolumn": twoColumnTable} {
		t.Run(name, func(t *testing.T) {
			h, r := renderedCells(md)
			natural := naturalTableWidth(h, r)
			for target := 30; target <= natural+10; target++ {
				h, r := renderedCells(md)
				out := renderTable(h, r, target)
				if got := tableWidth(out); got > target {
					t.Fatalf("target %d: rendered %d wide", target, got)
				}
				if !rightEdgeIntact(out) {
					t.Fatalf("target %d (natural %d): right edge cropped:\n%s", target, natural, out)
				}
			}
		})
	}
}

func TestRenderTableKeepsAllCellContentWhenWrapping(t *testing.T) {
	h, r := renderedCells(personaTable)
	out := stripEscapes(renderTable(h, r, 120))
	// The longest cell must survive wrapping, split across lines.
	for _, frag := range []string{"F. Power user on VPN: Apple (real_user)", "datacenter", "ASN flag"} {
		if !strings.Contains(out, frag) {
			t.Errorf("wrapped table dropped %q:\n%s", frag, out)
		}
	}
}
