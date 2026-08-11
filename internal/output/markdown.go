package output

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"golang.org/x/term"
)

// fjStyle is the in-code markdown style for fj. Tweak fields here to adjust
// how rendered markdown looks across all view commands.
//
// Color values are ANSI 256-color codes as strings (e.g. "36" = cyan, "242" =
// gray) — same convention glamour's built-in JSON styles use. To customize
// further, see charm.land/glamour/v2/ansi.StyleConfig for the full schema.
var fjStyle = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockPrefix: "\n",
			BlockSuffix: "\n",
		},
		Margin: new(uint(0)),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color:  new("242"),
			Italic: new(true),
		},
		Indent:      new(uint(1)),
		IndentToken: new("│ "),
	},
	Paragraph: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{},
	},
	List: ansi.StyleList{
		LevelIndent: 2,
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
	},

	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       new("39"),
			Bold:        new(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           new("228"),
			BackgroundColor: new("63"),
			Bold:            new(true),
		},
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "## ",
			Color:  new("39"),
			Bold:   new(true),
		},
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "### ",
			Color:  new("36"),
			Bold:   new(true),
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "#### ",
			Color:  new("36"),
		},
	},
	H5: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "##### ",
			Color:  new("36"),
		},
	},
	H6: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "###### ",
			Color:  new("36"),
			Bold:   new(false),
		},
	},

	Strikethrough: ansi.StylePrimitive{CrossedOut: new(true)},
	Emph:          ansi.StylePrimitive{Italic: new(true)},
	Strong:        ansi.StylePrimitive{Bold: new(true)},
	HorizontalRule: ansi.StylePrimitive{
		Color:  new("242"),
		Format: "\n--------\n",
	},

	Item:        ansi.StylePrimitive{BlockPrefix: "• "},
	Enumeration: ansi.StylePrimitive{BlockPrefix: ". "},
	Task: ansi.StyleTask{
		StylePrimitive: ansi.StylePrimitive{},
		Ticked:         "[✓] ",
		Unticked:       "[ ] ",
	},

	Link: ansi.StylePrimitive{
		Color:     new("36"),
		Underline: new(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: new("36"),
	},

	Image: ansi.StylePrimitive{
		Color:     new("36"),
		Underline: new(true),
	},
	ImageText: ansi.StylePrimitive{
		Color:  new("36"),
		Format: "Image: {{.text}} →",
	},

	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "`",
			Suffix: "`",
			Color:  new("203"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
			Margin:         new(uint(2)),
		},
		// Chroma (syntax highlighting) requires hex color strings, unlike
		// the rest of the style which uses ANSI 256-color codes.
		Chroma: &ansi.Chroma{
			Text:                ansi.StylePrimitive{Color: new("#C4C4C4")},
			Error:               ansi.StylePrimitive{Color: new("#F1F1F1"), BackgroundColor: new("#F05B5B")},
			Comment:             ansi.StylePrimitive{Color: new("#676767"), Italic: new(true)},
			CommentPreproc:      ansi.StylePrimitive{Color: new("#FF875F")},
			Keyword:             ansi.StylePrimitive{Color: new("#00AAFF")},
			KeywordReserved:     ansi.StylePrimitive{Color: new("#FF5FD2")},
			KeywordNamespace:    ansi.StylePrimitive{Color: new("#FF5FD2")},
			KeywordType:         ansi.StylePrimitive{Color: new("#EEAA00")},
			Operator:            ansi.StylePrimitive{Color: new("#EF8080")},
			Punctuation:         ansi.StylePrimitive{Color: new("#E8E8A8")},
			Name:                ansi.StylePrimitive{Color: new("#C4C4C4")},
			NameBuiltin:         ansi.StylePrimitive{Color: new("#FF8EC7")},
			NameTag:             ansi.StylePrimitive{Color: new("#B083EA")},
			NameAttribute:       ansi.StylePrimitive{Color: new("#7A7AE6")},
			NameClass:           ansi.StylePrimitive{Color: new("#F1F1F1"), Bold: new(true)},
			NameDecorator:       ansi.StylePrimitive{Color: new("#FFFF87")},
			NameFunction:        ansi.StylePrimitive{Color: new("#00D787")},
			LiteralNumber:       ansi.StylePrimitive{Color: new("#6EEFC0")},
			LiteralString:       ansi.StylePrimitive{Color: new("#C69669")},
			LiteralStringEscape: ansi.StylePrimitive{Color: new("#AFFFD7")},
			GenericDeleted:      ansi.StylePrimitive{Color: new("#FD5B5B")},
			GenericInserted:     ansi.StylePrimitive{Color: new("#00D787")},
			GenericEmph:         ansi.StylePrimitive{Italic: new(true)},
			GenericStrong:       ansi.StylePrimitive{Bold: new(true)},
			GenericSubheading:   ansi.StylePrimitive{Color: new("#777777")},
			Background:          ansi.StylePrimitive{BackgroundColor: new("#373737")},
		},
	},

	// Table separator characters. Glamour hardcodes the enclosing border off
	// and never draws a row separator between data rows, so we pre-extract
	// tables from the markdown and render them ourselves (see renderTable).
	// These rules remain as a fallback for any table that slips through.
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
		CenterSeparator: new("┼"),
		ColumnSeparator: new("│"),
		RowSeparator:    new("─"),
	},

	DefinitionList: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{}},
	DefinitionTerm: ansi.StylePrimitive{Bold: new(true)},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n  ",
	},

	HTMLBlock: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: new("242")}},
	HTMLSpan:  ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: new("242")}},
}

var (
	mdRendererOnce sync.Once
	mdRenderer     *glamour.TermRenderer
	mdRendererErr  error

	cellRendererOnce sync.Once
	cellRenderer     *glamour.TermRenderer
	cellRendererErr  error
)

func getMarkdownRenderer() (*glamour.TermRenderer, error) {
	mdRendererOnce.Do(func() {
		mdRenderer, mdRendererErr = glamour.NewTermRenderer(
			glamour.WithStyles(fjStyle),
			glamour.WithWordWrap(wrapWidth()),
		)
	})
	return mdRenderer, mdRendererErr
}

// getCellRenderer returns a renderer used for table cell content. Wrapping is
// disabled here — lipgloss.Table wraps cells itself based on the assigned
// column width.
func getCellRenderer() (*glamour.TermRenderer, error) {
	cellRendererOnce.Do(func() {
		cellRenderer, cellRendererErr = glamour.NewTermRenderer(
			glamour.WithStyles(fjStyle),
			glamour.WithWordWrap(0),
		)
	})
	return cellRenderer, cellRendererErr
}

func renderCellMarkdown(s string, linkIdx map[string]int) string {
	if s == "" {
		return s
	}
	s = compactLinksForCell(s, linkIdx)
	r, err := getCellRenderer()
	if err != nil {
		return s
	}
	out, err := r.Render(s)
	if err != nil {
		return s
	}
	return strings.TrimSpace(out)
}

var mdLinkRE = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\s]+)\)`)

// collectTableLinks scans every cell once and returns a url→1-based-index map
// plus the URLs in insertion order. Duplicates share a number so the footer
// list stays short.
func collectTableLinks(headers []string, rows [][]string) (map[string]int, []string) {
	idx := map[string]int{}
	var urls []string
	scan := func(s string) {
		for _, m := range mdLinkRE.FindAllStringSubmatch(s, -1) {
			url := m[2]
			if _, ok := idx[url]; ok {
				continue
			}
			urls = append(urls, url)
			idx[url] = len(urls)
		}
	}
	for _, h := range headers {
		scan(h)
	}
	for _, row := range rows {
		for _, c := range row {
			scan(c)
		}
	}
	return idx, urls
}

// compactLinksForCell rewrites `[text](url)` to `text[N]` wrapped in OSC 8
// hyperlink escapes, where N is the URL's footer index. Cells stay compact
// and the URL remains visible in the footer list printed under the table.
func compactLinksForCell(s string, linkIdx map[string]int) string {
	return mdLinkRE.ReplaceAllStringFunc(s, func(m string) string {
		parts := mdLinkRE.FindStringSubmatch(m)
		text, url := parts[1], parts[2]
		n := linkIdx[url]
		const linkColor = "\x1b[38;5;36m"
		const faint = "\x1b[38;5;242m"
		const reset = "\x1b[0m"
		return "\x1b]8;;" + url + "\x1b\\" +
			linkColor + text + reset +
			faint + fmt.Sprintf("[%d]", n) + reset +
			"\x1b]8;;\x1b\\"
	})
}

// renderLinkFooter formats the URL list that appears under a table.
func renderLinkFooter(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	const faint = "\x1b[38;5;242m"
	const link = "\x1b[38;5;36m"
	const reset = "\x1b[0m"
	var b strings.Builder
	for i, u := range urls {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s[%d]%s ", faint, i+1, reset)
		b.WriteString("\x1b]8;;" + u + "\x1b\\")
		b.WriteString(link + u + reset)
		b.WriteString("\x1b]8;;\x1b\\")
	}
	return b.String()
}

// wrapWidth returns the soft-wrap width for markdown rendering. It prefers
// the actual terminal width, capped at 120 chars (gh uses the same ceiling)
// so long lines stay comfortable to read on ultra-wide terminals. Users can
// override the cap via FJ_MDWIDTH; a non-TTY falls back to 80.
func wrapWidth() int {
	const fallback = 80
	cap := 120
	if v, err := strconv.Atoi(os.Getenv("FJ_MDWIDTH")); err == nil && v > 0 {
		cap = v
	}
	if !isTTY {
		if fallback > cap {
			return cap
		}
		return fallback
	}
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return fallback
	}
	if w > cap {
		return cap
	}
	return w
}

// RenderMarkdown returns a styled rendering of body suitable for the current
// output target. When stdout is not a TTY (piped, redirected, etc.) the input
// is returned verbatim so machine consumers and agents see clean markdown.
//
// On any rendering error the raw body is returned unchanged.
func RenderMarkdown(body string) string {
	if body == "" {
		return body
	}
	if !isTTY {
		return body
	}
	r, err := getMarkdownRenderer()
	if err != nil {
		return body
	}
	// Pull tables out of the source and render them with lipgloss directly —
	// glamour hardcodes enclosing borders off and skips per-row separators.
	preprocessed, tables := extractTables(body)
	out, err := r.Render(preprocessed)
	if err != nil {
		return body
	}
	for i, rendered := range tables {
		out = strings.Replace(out, tableSentinel(i), rendered, 1)
	}
	return out
}

func tableSentinel(i int) string {
	return fmt.Sprintf("FJTBL%dZZ", i)
}

// extractTables walks the markdown source line-by-line, splicing out GitHub
// Flavored Markdown tables and replacing each with a short sentinel. The
// rendered tables are returned in order so RenderMarkdown can swap the
// sentinels back in after glamour runs.
func extractTables(src string) (string, []string) {
	lines := strings.Split(src, "\n")
	var tables []string
	var out strings.Builder
	for i := 0; i < len(lines); {
		if i+1 < len(lines) && isTableRow(lines[i]) && isTableSeparator(lines[i+1]) {
			end := i + 2
			for end < len(lines) && isTableRow(lines[end]) {
				end++
			}
			header := parseTableRow(lines[i])
			rows := make([][]string, 0, end-(i+2))
			for _, ln := range lines[i+2 : end] {
				rows = append(rows, parseTableRow(ln))
			}
			linkIdx, urls := collectTableLinks(header, rows)
			for j, cell := range header {
				header[j] = renderCellMarkdown(cell, linkIdx)
			}
			for _, row := range rows {
				for j, cell := range row {
					row[j] = renderCellMarkdown(cell, linkIdx)
				}
			}
			tables = append(tables, renderTable(header, rows, wrapWidth())+renderLinkFooter(urls))
			out.WriteString("\n")
			out.WriteString(tableSentinel(len(tables) - 1))
			out.WriteString("\n")
			i = end
			continue
		}
		out.WriteString(lines[i])
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
		i++
	}
	return out.String(), tables
}

func isTableRow(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) >= 2 && strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|")
}

func isTableSeparator(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 3 || !strings.HasPrefix(s, "|") || !strings.HasSuffix(s, "|") {
		return false
	}
	cells := strings.Split(strings.Trim(s, "|"), "|")
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false
		}
		for _, c := range cell {
			if c != '-' && c != ':' {
				return false
			}
		}
	}
	return true
}

func parseTableRow(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "|")
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func renderTable(headers []string, rows [][]string, maxWidth int) string {
	border := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Padding(0, 1)
	cell := lipgloss.NewStyle().Padding(0, 1)
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(border).
		BorderRow(true).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return header
			}
			return cell
		})
	// Only pin a width (and enable wrapping) when the natural table would
	// overflow the target — otherwise short columns get needlessly padded.
	if natural := naturalTableWidth(headers, rows); natural > maxWidth {
		t.Width(shrinkTarget(natural, len(headers), maxWidth)).Wrap(true)
	}
	return t.String()
}

// shrinkTarget picks the width to hand lipgloss so it actually shrinks columns.
//
// lipgloss v2.0.5 decides between growing and shrinking by comparing the
// summed column widths against the requested width — but that sum omits the
// border characters, while the final render is hard-cropped to the requested
// width. So for an overflow smaller than the border budget it takes the grow
// path, leaves the columns alone, and crops the right edge off the table
// instead of wrapping. Asking for one column less than the sum lipgloss
// compares against keeps it on the shrink path.
func shrinkTarget(natural, cols, maxWidth int) int {
	border := cols + 1 // one separator per column, plus the leading edge
	if forceShrink := natural - border - 1; forceShrink < maxWidth {
		return forceShrink
	}
	return maxWidth
}

// naturalTableWidth estimates how wide the table would render with no
// wrapping: max visible content width per column + 2 chars of cell padding +
// 1 column separator per column + 1 leading border.
func naturalTableWidth(headers []string, rows [][]string) int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		if w := lipgloss.Width(h); w > widths[i] {
			widths[i] = w
		}
	}
	for _, row := range rows {
		for i, c := range row {
			if i >= len(widths) {
				break
			}
			for line := range strings.SplitSeq(c, "\n") {
				if w := lipgloss.Width(line); w > widths[i] {
					widths[i] = w
				}
			}
		}
	}
	total := 1 // leading border
	for _, w := range widths {
		total += w + 3 // 2 padding + 1 separator/trailing border
	}
	return total
}
