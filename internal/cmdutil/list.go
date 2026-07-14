package cmdutil

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// sortAliases maps the friendly --sort values fj accepts to the sort keys the
// Forgejo API understands. An empty API value means "server default" (newest
// first), which needs no query parameter.
var sortAliases = map[string]string{
	"newest":                 "",
	"oldest":                 "oldest",
	"updated":                "recentupdate",
	"recently-updated":       "recentupdate",
	"least-recently-updated": "leastupdate",
	"most-commented":         "mostcomment",
	"least-commented":        "leastcomment",
	"priority":               "priority",
}

// SortValue resolves a user-provided --sort value to a Forgejo API sort key.
// It accepts the friendly aliases above (and the raw API keys). An empty input
// yields an empty key (server default). Unknown values return a flag error
// listing the valid options.
func SortValue(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return "", nil
	}
	if api, ok := sortAliases[v]; ok {
		return api, nil
	}
	// Allow raw Forgejo API keys to pass through unchanged.
	for _, api := range sortAliases {
		if api == v {
			return v, nil
		}
	}
	return "", FlagErrorf("invalid --sort value %q (valid: %s)", v, strings.Join(sortValues(), ", "))
}

// sortValues returns the accepted friendly --sort values, sorted for stable
// help/error output.
func sortValues() []string {
	vals := make([]string, 0, len(sortAliases))
	for k := range sortAliases {
		vals = append(vals, k)
	}
	sort.Strings(vals)
	return vals
}

// ParseTimeFilter parses a --since/--before style value into an absolute time.
// It accepts an RFC3339 timestamp, a plain YYYY-MM-DD date, or a relative age
// like "7d", "24h", "2w", "30m" interpreted as "that long ago". An empty input
// yields the zero time (meaning "no filter").
func ParseTimeFilter(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, nil
	}
	if d, ok := parseRelativeAge(v); ok {
		return time.Now().Add(-d), nil
	}
	return time.Time{}, FlagErrorf(
		"invalid time %q (use YYYY-MM-DD, an RFC3339 timestamp, or a relative age like 7d)", v,
	)
}

// parseRelativeAge parses durations Go's time.ParseDuration rejects — days and
// weeks — falling back to time.ParseDuration for h/m/s units.
func parseRelativeAge(v string) (time.Duration, bool) {
	if len(v) >= 2 {
		unit := v[len(v)-1]
		if n, err := strconv.Atoi(v[:len(v)-1]); err == nil && n >= 0 {
			switch unit {
			case 'd':
				return time.Duration(n) * 24 * time.Hour, true
			case 'w':
				return time.Duration(n) * 7 * 24 * time.Hour, true
			}
		}
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d, true
	}
	return 0, false
}
