package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/gojq"
)

// ExportJSON writes v the way gh's --json does: indented on a terminal and
// compact on a pipe, then through jq or tmpl when one is given. Object keys
// come out sorted because v is built from maps.
func ExportJSON(w io.Writer, v any, jq *gojq.Code, tmpl *Template) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if isTTY {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		return err
	}
	switch {
	case jq != nil:
		return WriteJQ(w, jq, buf.Bytes())
	case tmpl != nil:
		return tmpl.Execute(w, buf.Bytes())
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// Template is a parsed --template: a Go template run over a JSON document,
// with gh's helper functions.
type Template struct {
	tmpl *template.Template
	w    io.Writer
	rows [][]string
}

var ansiStyles = map[string]string{
	"black": "\033[30m", "red": Red, "green": Green, "yellow": Yellow, "blue": Blue,
	"magenta": Magenta, "cyan": Cyan, "white": "\033[37m", "gray": Gray, "bold": Bold,
}

// ParseTemplate parses text with gh's template functions. color takes a
// plain color name; the modifiers after "+" or ":" in gh's styles are ignored.
func ParseTemplate(text string) (*Template, error) {
	t := &Template{}
	color := func(style string, v any) string {
		name, _, _ := strings.Cut(style, "+")
		name, _, _ = strings.Cut(name, ":")
		if code, ok := ansiStyles[name]; ok {
			return code + scalar(v) + Reset
		}
		return scalar(v)
	}
	parseTime := func(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }
	funcs := template.FuncMap{
		"autocolor": func(style string, v any) string {
			if !isTTY {
				return scalar(v)
			}
			return color(style, v)
		},
		"color": color,
		"join": func(sep string, list []any) string {
			s := make([]string, len(list))
			for i, v := range list {
				s[i] = scalar(v)
			}
			return strings.Join(s, sep)
		},
		"pluck": func(field string, list []any) []any {
			var out []any
			for _, v := range list {
				if m, ok := v.(map[string]any); ok {
					out = append(out, m[field])
				}
			}
			return out
		},
		"tablerow": func(fields ...any) string {
			row := make([]string, len(fields))
			for i, v := range fields {
				row[i] = scalar(v)
			}
			t.rows = append(t.rows, row)
			return ""
		},
		"tablerender": func() string {
			t.flush()
			return ""
		},
		"timeago": func(s string) (string, error) {
			tm, err := parseTime(s)
			return RelativeTimeStr(tm), err
		},
		"timefmt": func(format, s string) (string, error) {
			tm, err := parseTime(s)
			return tm.Format(format), err
		},
		"truncate": func(n int, v any) string { return TruncateDisplay(scalar(v), n) },
		"hyperlink": func(link, text string) string {
			if text == "" {
				text = link
			}
			if !isTTY {
				return text
			}
			return "\033]8;;" + link + "\033\\" + text + "\033]8;;\033\\"
		},
		"contains":   func(sub, s string) bool { return strings.Contains(s, sub) },
		"hasPrefix":  func(prefix, s string) bool { return strings.HasPrefix(s, prefix) },
		"hasSuffix":  func(suffix, s string) bool { return strings.HasSuffix(s, suffix) },
		"regexMatch": func(re, s string) (bool, error) { return regexp.MatchString(re, s) },
	}
	tmpl, err := template.New("").Funcs(funcs).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	t.tmpl = tmpl
	return t, nil
}

// Execute runs the template over the JSON document in input, then prints
// any tablerow rows that no tablerender flushed. Numbers decode as
// json.Number so large IDs print whole instead of in exponent form.
func (t *Template) Execute(w io.Writer, input []byte) error {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	t.w = w
	if err := t.tmpl.Execute(w, v); err != nil {
		return err
	}
	t.flush()
	return nil
}

// flush writes the pending tablerow rows as columns two spaces apart.
func (t *Template) flush() {
	var widths []int
	for _, row := range t.rows {
		for i, cell := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], DisplayWidth(cell))
		}
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(row)-1 {
				cell += strings.Repeat(" ", widths[i]-DisplayWidth(cell)+tablePadding)
			}
			fmt.Fprint(t.w, cell)
		}
		fmt.Fprintln(t.w)
	}
	t.rows = nil
}

// scalar renders a JSON value as template text, with null as "".
func scalar(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
