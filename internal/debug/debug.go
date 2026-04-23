// Package debug provides leveled debug logging controlled by the DEBUG
// environment variable.
//
//	DEBUG=1  high-level phases
//	DEBUG=2  individual API calls with timing
//	DEBUG=3  DEBUG=2 + request URLs, page numbers, and response counts
package debug

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

var level = parseLevel(os.Getenv("DEBUG"))

func parseLevel(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// Level returns the current debug level.
func Level() int { return level }

// Enabled reports whether logs at level l would be emitted.
func Enabled(l int) bool { return level >= l }

// Logf writes a debug line to stderr if the current level is at least l.
func Logf(l int, format string, args ...any) {
	if level < l {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(os.Stderr, "[DEBUG%d %s] ", l, ts)
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

// Track logs "→ name" immediately and returns a function that logs
// "← name (duration)" when called. It is a no-op if level < l.
//
//	defer debug.Track(2, "ListRepoIssues open")()
func Track(l int, name string) func() {
	if level < l {
		return func() {}
	}
	start := time.Now()
	Logf(l, "-> %s", name)
	return func() {
		Logf(l, "<- %s (%s)", name, time.Since(start))
	}
}
