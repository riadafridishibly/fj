package cmdutil

import (
	"sort"
	"strings"
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
