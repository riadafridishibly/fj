package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
type Table struct {
	headers []string
	rows    [][]string
	padding int
}

func NewTable(headers ...string) *Table {
	return &Table{
		headers: headers,
		padding: 2,
	}
}

func (t *Table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
}

func (t *Table) Render(w io.Writer) {
	numCols := len(t.headers)
	widths := make([]int, numCols)

	// Measure header widths
	for i, h := range t.headers {
		widths[i] = displayWidth(h)
	}

	// Measure row widths
	for _, row := range t.rows {
		for i := range min(len(row), numCols) {
			if dw := displayWidth(row[i]); dw > widths[i] {
				widths[i] = dw
			}
		}
	}

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
			fmt.Fprint(w, row[i])
			if i < numCols-1 {
				pad := widths[i] - displayWidth(row[i])
				if pad > 0 {
					fmt.Fprint(w, strings.Repeat(" ", pad))
				}
			}
		}
		fmt.Fprintln(w)
	}
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
