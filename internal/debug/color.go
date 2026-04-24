package debug

import (
	"bytes"
	"encoding/json"
	"os"

	"golang.org/x/term"

	"github.com/riadafridishibly/fj/internal/output"
)

// colorEnabled reports whether ANSI codes should be emitted on stderr.
// Honors NO_COLOR (https://no-color.org) and otherwise requires a TTY.
var colorEnabled = func() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}()

// paint wraps s in code + reset when coloring is on.
func paint(code, s string) string {
	if !colorEnabled || s == "" {
		return s
	}
	return code + s + output.Reset
}

// methodColor picks an ANSI color code for an HTTP method verb.
func methodColor(method string) string {
	switch method {
	case "GET":
		return output.Cyan
	case "POST":
		return output.Yellow
	case "PUT", "PATCH":
		return output.Magenta
	case "DELETE":
		return output.Red
	default:
		return output.Blue
	}
}

// statusColor picks an ANSI color code for an HTTP status code by class.
func statusColor(code int) string {
	switch {
	case code >= 500:
		return output.Red
	case code >= 400:
		return output.Yellow
	case code >= 300:
		return output.Blue
	case code >= 200:
		return output.Green
	default:
		return output.Dim
	}
}

// prettyJSON pretty-prints raw JSON bytes. Returns the original bytes
// unchanged if src is not valid JSON.
func prettyJSON(src []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, src, "", "  "); err != nil {
		return src
	}
	return buf.Bytes()
}

// colorizeJSON returns pretty JSON with ANSI colors applied to keys,
// strings, numbers, booleans, and null. Assumes input is already valid,
// pretty-printed JSON (whitespace between tokens is preserved verbatim).
// Falls back to src unchanged when coloring is disabled.
func colorizeJSON(src []byte) []byte {
	if !colorEnabled {
		return src
	}
	var out bytes.Buffer
	out.Grow(len(src) + len(src)/4)
	i := 0
	for i < len(src) {
		b := src[i]
		switch {
		case b == '"':
			j := scanString(src, i)
			// A '"' outside any string can only be followed by ':' (key) or
			// one of ',}]' / whitespace (string value) — valid JSON grammar.
			k := j
			for k < len(src) && (src[k] == ' ' || src[k] == '\t') {
				k++
			}
			if k < len(src) && src[k] == ':' {
				out.WriteString(output.Blue)
			} else {
				out.WriteString(output.Green)
			}
			out.Write(src[i:j])
			out.WriteString(output.Reset)
			i = j
		case b == '-' || (b >= '0' && b <= '9'):
			j := scanNumber(src, i)
			out.WriteString(output.Yellow)
			out.Write(src[i:j])
			out.WriteString(output.Reset)
			i = j
		case b == 't' && hasPrefixAt(src, i, "true"):
			out.WriteString(output.Bold)
			out.WriteString("true")
			out.WriteString(output.Reset)
			i += 4
		case b == 'f' && hasPrefixAt(src, i, "false"):
			out.WriteString(output.Bold)
			out.WriteString("false")
			out.WriteString(output.Reset)
			i += 5
		case b == 'n' && hasPrefixAt(src, i, "null"):
			out.WriteString(output.Dim)
			out.WriteString("null")
			out.WriteString(output.Reset)
			i += 4
		default:
			out.WriteByte(b)
			i++
		}
	}
	return out.Bytes()
}

func scanString(src []byte, start int) int {
	for j := start + 1; j < len(src); {
		switch src[j] {
		case '\\':
			if j+1 >= len(src) {
				return len(src)
			}
			j += 2
		case '"':
			return j + 1
		default:
			j++
		}
	}
	return len(src)
}

func scanNumber(src []byte, start int) int {
	j := start
	for j < len(src) {
		switch c := src[j]; {
		case c >= '0' && c <= '9', c == '-', c == '+', c == '.', c == 'e', c == 'E':
			j++
		default:
			return j
		}
	}
	return j
}

func hasPrefixAt(src []byte, i int, s string) bool {
	if i+len(s) > len(src) {
		return false
	}
	return string(src[i:i+len(s)]) == s
}
