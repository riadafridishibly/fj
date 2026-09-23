package cmdutil

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/output"
)

// JSONFlags holds gh's formatting flags: --json <fields>, -q/--jq and
// -t/--template. Field names and shapes follow gh's, not Forgejo's.
type JSONFlags struct {
	Fields   []string
	jq       string
	template string
	code     *gojq.Code
	tmpl     *output.Template
}

// AddJSONFlags adds --json and -q/--jq to cmd, plus -t/--template when
// withTemplate is set. Write commands leave the template flag out, since -t
// is --title on create and edit. fields are gh's names for the command's data;
// fjFields are fj's additions, listed apart in help. It takes over
// cmd.PreRunE to validate the flags.
func AddJSONFlags(cmd *cobra.Command, j *JSONFlags, fields, fjFields []string, withTemplate bool) {
	cmd.Flags().StringSliceVar(&j.Fields, "json", nil, "Output JSON with the specified `fields` (gh field names)")
	cmd.Flags().StringVarP(&j.jq, "jq", "q", "", "Filter JSON output using a jq `expression`")
	if withTemplate {
		cmd.Flags().StringVarP(&j.template, "template", "t", "", `Format JSON output using a Go template; see "fj help formatting"`)
	}

	all := slices.Sorted(slices.Values(append(slices.Clone(fields), fjFields...)))
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		if err.Error() == "flag needs an argument: --json" {
			return fmt.Errorf("Specify one or more comma-separated fields for `--json`:\n  %s", strings.Join(all, "\n  "))
		}
		if c.HasParent() {
			return c.Parent().FlagErrorFunc()(c, err)
		}
		return err
	})
	cmd.PreRunE = func(c *cobra.Command, _ []string) error {
		return j.check(c.Flags(), all)
	}
	help := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		help(c, args)
		fmt.Fprint(c.OutOrStdout(), fieldsHelp(fields, fjFields))
	})
}

// check validates the flags the way gh does and compiles --jq or --template,
// so a bad expression fails before any request is sent.
func (j *JSONFlags) check(flags *pflag.FlagSet, all []string) error {
	if !flags.Changed("json") {
		if flags.Changed("jq") {
			return errors.New("cannot use `--jq` without specifying `--json`")
		}
		if flags.Changed("template") {
			return errors.New("cannot use `--template` without specifying `--json`")
		}
		return nil
	}
	if flags.Changed("web") {
		return errors.New("cannot use `--web` with `--json`")
	}
	for _, f := range j.Fields {
		if !slices.Contains(all, f) {
			return fmt.Errorf("Unknown JSON field: %q\nAvailable fields:\n  %s", f, strings.Join(all, "\n  "))
		}
	}
	slices.Sort(j.Fields)
	j.Fields = slices.Compact(j.Fields)

	var err error
	if j.jq != "" {
		j.code, err = output.ParseJQ(j.jq)
	} else if j.template != "" {
		j.tmpl, err = output.ParseTemplate(j.template)
	}
	return err
}

// Enabled reports whether --json was given. pflag sets Fields to a non-nil
// slice even for an empty value.
func (j *JSONFlags) Enabled() bool { return j.Fields != nil }

// Has reports whether field was requested, for data that costs a request.
func (j *JSONFlags) Has(field string) bool { return slices.Contains(j.Fields, field) }

// Write keeps the requested fields of data and prints it. data is one object
// or a slice of them, keyed by field name; a nil slice prints as [].
func (j *JSONFlags) Write(w io.Writer, data any) error {
	switch d := data.(type) {
	case map[string]any:
		data = j.pick(d)
	case []map[string]any:
		out := make([]map[string]any, len(d))
		for i, m := range d {
			out[i] = j.pick(m)
		}
		data = out
	}
	return output.ExportJSON(w, data, j.code, j.tmpl)
}

func (j *JSONFlags) pick(m map[string]any) map[string]any {
	out := make(map[string]any, len(j.Fields))
	for _, f := range j.Fields {
		if v, ok := m[f]; ok {
			out[f] = v
		}
	}
	return out
}

// fieldsHelp is the JSON FIELDS section appended to a command's help. A
// command gh lacks has no gh fields; its fj fields are then listed alone.
func fieldsHelp(fields, fjFields []string) string {
	var b strings.Builder
	if len(fields) == 0 {
		b.WriteString("\nJSON FIELDS\n  gh (GitHub CLI) has no such command; the names follow gh's style.\n")
		fields, fjFields = fjFields, nil
	} else {
		b.WriteString("\nJSON FIELDS\n  Names and shapes follow gh (GitHub CLI); fields Forgejo lacks are omitted.\n")
	}
	line := " "
	for i, f := range fields {
		if i < len(fields)-1 {
			f += ","
		}
		if len(line)+1+len(f) > 80 {
			b.WriteString(line + "\n")
			line = " "
		}
		line += " " + f
	}
	b.WriteString(line + "\n")
	if len(fjFields) > 0 {
		b.WriteString("  fj-only: " + strings.Join(fjFields, ", ") + "\n")
	}
	return b.String()
}

// JSONTime formats t as gh does, RFC 3339 in UTC, or nil when t is unset.
func JSONTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// JSONUser is gh's user shape, {id, login, name}.
func JSONUser(u *forgejo.User) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{"id": u.ID, "login": u.UserName, "name": u.FullName}
}

// JSONUsers is JSONUser over a list, [] when empty.
func JSONUsers(users []*forgejo.User) []map[string]any {
	out := make([]map[string]any, len(users))
	for i, u := range users {
		out[i] = JSONUser(u)
	}
	return out
}

// JSONLabels is gh's label shape, with the color's leading "#" dropped.
func JSONLabels(labels []*forgejo.Label) []map[string]any {
	out := make([]map[string]any, len(labels))
	for i, l := range labels {
		out[i] = map[string]any{
			"id": l.ID, "name": l.Name, "description": l.Description,
			"color": strings.TrimPrefix(l.Color, "#"),
		}
	}
	return out
}

// JSONMilestone is gh's milestone shape. gh's number is Forgejo's milestone
// ID, which is what fj's milestone commands accept alongside the title.
func JSONMilestone(m *forgejo.Milestone) map[string]any {
	if m == nil {
		return nil
	}
	return map[string]any{
		"number": m.ID, "title": m.Title, "description": m.Description,
		"dueOn": JSONTime(m.Deadline),
	}
}

// JSONComments lists the comments on issue or pull request index in gh's
// comment shape.
func JSONComments(client *api.Client, repo Repo, index int64) ([]map[string]any, error) {
	var comments []*forgejo.Comment
	if err := client.GetJSON(repo.APIPath("/issues/%d/comments", index), &comments); err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}
	out := make([]map[string]any, len(comments))
	for i, c := range comments {
		var login string
		if c.Poster != nil {
			login = c.Poster.UserName
		}
		out[i] = map[string]any{
			"id": c.ID, "author": map[string]any{"login": login}, "body": c.Body,
			"createdAt": JSONTime(&c.Created), "url": c.HTMLURL,
		}
	}
	return out, nil
}
