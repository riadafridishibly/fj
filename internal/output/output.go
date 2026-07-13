package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

// Color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Cyan    = "\033[36m"
	Magenta = "\033[35m"
	Gray    = "\033[38;5;242m"
)

var isTTY = term.IsTerminal(int(os.Stdout.Fd()))

// IsTerminal returns true if stdout is a terminal
func IsTerminal() bool {
	return isTTY
}

// Table layout width variables. These govern how much horizontal space a
// rendered Table is allowed to use and how far flexible columns may shrink.
const (
	// tablePadding is the number of spaces printed between columns.
	tablePadding = 2
	// minFlexWidth is the smallest width a flexible column may shrink to
	// before the table stops reclaiming space from it (it may still be
	// floored higher by its header width).
	minFlexWidth = 12
)

// TerminalWidth returns the usable width budget for tables:
//   - the FJ_WIDTH environment override, if set to a positive integer;
//   - otherwise the current terminal width, when stdout is a TTY;
//   - otherwise 0, meaning "unbounded" (e.g. when piped) so nothing is
//     truncated and downstream tools receive full content.
func TerminalWidth() int {
	if v := os.Getenv("FJ_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if !isTTY {
		return 0
	}
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 0
	}
	return w
}

// Colorize wraps text with an ANSI color code, only if stdout is a TTY
func Colorize(color, text string) string {
	if !isTTY || text == "" {
		return text
	}
	return color + text + Reset
}

func NewTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func Stdout() io.Writer {
	return os.Stdout
}

func Stderr() io.Writer {
	return os.Stderr
}

func RelativeTimeStr(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "less than a minute ago"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "about 1 minute ago"
		}
		return fmt.Sprintf("about %d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "about 1 hour ago"
		}
		return fmt.Sprintf("about %d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "about 1 day ago"
		}
		return fmt.Sprintf("about %d days ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months <= 1 {
			return "about 1 month ago"
		}
		return fmt.Sprintf("about %d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "about 1 year ago"
		}
		return fmt.Sprintf("about %d years ago", years)
	}
}

// Sanitize removes newlines and excess whitespace from a string,
// making it safe for tabwriter output.
func Sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

func Truncate(s string, maxLen int) string {
	s = Sanitize(s)
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func LabelNames(labels any) string {
	switch v := labels.(type) {
	case []string:
		return strings.Join(v, ", ")
	default:
		return ""
	}
}

func PrintHeader(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format+"\n", args...)
}

func PrintField(w io.Writer, label, value string) {
	fmt.Fprintf(w, "%s:\t%s\n", label, value)
}

// Table prints aligned columns with proper ANSI color support.
// Unlike tabwriter, it calculates column widths based on display width
// (stripping ANSI codes) so colors don't break alignment.
//
// Columns marked via Flexible absorb the leftover terminal width and are
// truncated with an ellipsis only when a row would otherwise overflow the
// terminal. When the width budget is unbounded (piped output), nothing is
// truncated.
type Table struct {
	headers  []string
	rows     [][]string
	padding  int
	maxWidth int
	flexCols []int
	minFlex  int
}

func NewTable(headers ...string) *Table {
	return &Table{
		headers:  headers,
		padding:  tablePadding,
		maxWidth: TerminalWidth(),
		minFlex:  minFlexWidth,
	}
}

// Flexible marks the given column indices as flexible: they yield width to
// keep the table within its budget and get truncated with an ellipsis when
// space runs short. Columns are listed in priority order — the first argument
// is protected the longest, and later columns give up their width first. So
// Flexible(titleCol, labelsCol) keeps titles wide and trims labels first.
// Returns the table for chaining.
func (t *Table) Flexible(cols ...int) *Table {
	t.flexCols = cols
	return t
}

// MaxWidth overrides the width budget (0 = unbounded). Returns the table
// for chaining.
func (t *Table) MaxWidth(w int) *Table {
	t.maxWidth = w
	return t
}

func (t *Table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
}

func (t *Table) Render(w io.Writer) {
	numCols := len(t.headers)
	widths := t.naturalWidths()

	// Shrink flexible columns so the row fits the width budget.
	flex := t.fitFlexible(widths)

	// Print header
	for i, h := range t.headers {
		if i > 0 {
			fmt.Fprint(w, strings.Repeat(" ", t.padding))
		}
		fmt.Fprint(w, Colorize(Bold, h))
		if i < numCols-1 {
			pad := widths[i] - displayWidth(h)
			if pad > 0 {
				fmt.Fprint(w, strings.Repeat(" ", pad))
			}
		}
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range t.rows {
		for i := range min(len(row), numCols) {
			if i > 0 {
				fmt.Fprint(w, strings.Repeat(" ", t.padding))
			}
			cell := row[i]
			if flex[i] {
				cell = truncateDisplay(cell, widths[i])
			}
			fmt.Fprint(w, cell)
			if i < numCols-1 {
				pad := widths[i] - displayWidth(cell)
				if pad > 0 {
					fmt.Fprint(w, strings.Repeat(" ", pad))
				}
			}
		}
		fmt.Fprintln(w)
	}
}

// naturalWidths returns the untruncated display width of each column.
func (t *Table) naturalWidths() []int {
	numCols := len(t.headers)
	widths := make([]int, numCols)
	for i, h := range t.headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range t.rows {
		for i := range min(len(row), numCols) {
			if dw := displayWidth(row[i]); dw > widths[i] {
				widths[i] = dw
			}
		}
	}
	return widths
}

// fitFlexible shrinks flexible columns in-place until the total row width fits
// t.maxWidth (when bounded). It reclaims from the lowest-priority flexible
// column first (the last one passed to Flexible), fully exhausting it down to
// its floor before touching the next, so the highest-priority column (e.g. the
// title) keeps its full width for as long as possible. It returns a lookup of
// which columns are flexible (and thus truncated at render time).
func (t *Table) fitFlexible(widths []int) map[int]bool {
	numCols := len(t.headers)
	flex := make(map[int]bool, len(t.flexCols))
	valid := make([]int, 0, len(t.flexCols))
	for _, c := range t.flexCols {
		if c < 0 || c >= numCols {
			continue
		}
		flex[c] = true
		valid = append(valid, c)
	}

	if t.maxWidth <= 0 || len(valid) == 0 {
		return flex
	}

	total := (numCols - 1) * t.padding
	for _, w := range widths {
		total += w
	}
	overflow := total - t.maxWidth

	// Reclaim from lowest priority (last listed) to highest (first listed).
	for i := len(valid) - 1; i >= 0 && overflow > 0; i-- {
		c := valid[i]
		// Never shrink below the header width or minFlex.
		floor := t.minFlex
		if hw := displayWidth(t.headers[c]); hw > floor {
			floor = hw
		}
		if reducible := widths[c] - floor; reducible > 0 {
			take := min(reducible, overflow)
			widths[c] -= take
			overflow -= take
		}
	}
	return flex
}

// displayWidth returns the visible width of a string, ignoring ANSI escape codes.
func displayWidth(s string) int {
	n := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		if r == '\033' {
			inEscape = true
			continue
		}
		n++
	}
	return n
}

// truncateDisplay shortens s to at most maxWidth display columns, appending a
// single-column ellipsis when it has to cut. ANSI escape sequences are copied
// through without counting toward the width, and a Reset is appended if the
// string contained color so a mid-color cut doesn't bleed into later columns.
func truncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if displayWidth(s) <= maxWidth {
		return s
	}
	const ellipsis = "…"
	limit := maxWidth - 1 // reserve one column for the ellipsis
	var b strings.Builder
	n := 0
	inEscape := false
	hadColor := false
	for _, r := range s {
		if inEscape {
			b.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		if r == '\033' {
			inEscape = true
			hadColor = true
			b.WriteRune(r)
			continue
		}
		if n >= limit {
			break
		}
		b.WriteRune(r)
		n++
	}
	b.WriteString(ellipsis)
	if hadColor {
		b.WriteString(Reset)
	}
	return b.String()
}
