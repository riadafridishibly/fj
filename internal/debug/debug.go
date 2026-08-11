// Package debug provides leveled debug logging controlled by the DEBUG
// environment variable.
//
//	DEBUG=1  high-level phases
//	DEBUG=2  individual API calls with timing
//	DEBUG=3  DEBUG=2 + request URLs, X-Total-Count, and request/response bodies
package debug

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/riadafridishibly/fj/internal/output"
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

// Enabled reports whether logs at level l would be emitted.
func Enabled(l int) bool { return level >= l }

// Logf writes a debug line to stderr if the current level is at least l.
func Logf(l int, format string, args ...any) {
	if level < l {
		return
	}
	writePrefix(l)
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

// Body writes a label line followed by the body on subsequent lines,
// indented for readability. JSON payloads are pretty-printed and
// colorized when stderr is a TTY. No-op when level < l.
func Body(l int, label string, data []byte, contentType string) {
	if level < l || len(data) == 0 {
		return
	}
	writePrefix(l)
	fmt.Fprintln(os.Stderr, label)

	body := data
	if isJSON(contentType) {
		body = colorizeJSON(prettyJSON(data))
	}
	writeIndented(os.Stderr, body, "  ")
}

func writePrefix(l int) {
	ts := time.Now().Format("15:04:05.000")
	prefix := fmt.Sprintf("[DEBUG%d %s] ", l, ts)
	fmt.Fprint(os.Stderr, paint(output.Dim, prefix))
}

func writeIndented(w io.Writer, data []byte, indent string) {
	for len(data) > 0 {
		io.WriteString(w, indent)
		nl := bytes.IndexByte(data, '\n')
		if nl < 0 {
			w.Write(data)
			fmt.Fprintln(w)
			return
		}
		w.Write(data[:nl+1])
		data = data[nl+1:]
	}
}

// Track logs "→ name" immediately and returns a function that logs
// "← name (duration)" when called. It is a no-op if level < l.
//
//	defer debug.Track(1, "fj status (total)")()
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

// mediaType returns the canonical lowercase media type (e.g. "application/json")
// stripped of parameters, or "" if ct is empty or unparseable.
func mediaType(ct string) string {
	if ct == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return ""
	}
	return mt
}

// isJSON reports whether the given Content-Type is a JSON media type.
func isJSON(ct string) bool {
	mt := mediaType(ct)
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}

// isBinary reports whether the given Content-Type should be treated as
// opaque bytes (skipped in body logging). Empty/unknown types default to
// non-binary so unannotated JSON responses still log.
func isBinary(ct string) bool {
	mt := mediaType(ct)
	if mt == "" {
		return false
	}
	switch {
	case strings.HasPrefix(mt, "text/"),
		strings.HasSuffix(mt, "+json"),
		strings.HasSuffix(mt, "+xml"),
		mt == "application/json",
		mt == "application/xml",
		mt == "application/x-www-form-urlencoded":
		return false
	}
	return true
}
