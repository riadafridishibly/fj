package cmdutil

import "strings"

// Excerpt renders the first line of a body string, trimmed and truncated for
// one-line display (e.g. when showing the target of a delete command).
func Excerpt(body string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	if len(line) > 80 {
		line = line[:77] + "..."
	}
	return line
}
