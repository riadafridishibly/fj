// Package comment implements the `fj pr review comment` subcommands for
// managing inline review comments on a pull request.
package comment

import (
	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <command>",
		Short: "Manage inline review comments on a pull request",
	}
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdReply(f))
	cmd.AddCommand(NewCmdDelete(f))
	return cmd
}

// commentFields name an inline comment's JSON fields. gh has no command for
// these, so the names follow GitHub's PullRequestReviewComment.
var commentFields = []string{
	"author", "body", "commit", "createdAt", "diffHunk", "id", "line", "originalCommit",
	"originalLine", "path", "resolvedBy", "reviewId", "updatedAt", "url",
}

// JSON returns c keyed by field name. line and originalLine are Forgejo's
// position and original_position: the line in the new and the old file.
func JSON(c *forgejo.PullReviewComment) map[string]any {
	var login string
	if c.Reviewer != nil {
		login = c.Reviewer.UserName
	}
	var resolvedBy map[string]any
	if c.Resolver != nil {
		resolvedBy = map[string]any{"login": c.Resolver.UserName}
	}
	return map[string]any{
		"author":         map[string]any{"login": login},
		"body":           c.Body,
		"commit":         map[string]any{"oid": c.CommitID},
		"createdAt":      cmdutil.JSONTime(&c.Created),
		"diffHunk":       c.DiffHunk,
		"id":             c.ID,
		"line":           c.LineNum,
		"originalCommit": map[string]any{"oid": c.OrigCommitID},
		"originalLine":   c.OldLineNum,
		"path":           c.Path,
		"resolvedBy":     resolvedBy,
		"reviewId":       c.ReviewID,
		"updatedAt":      cmdutil.JSONTime(&c.Updated),
		"url":            c.HTMLURL,
	}
}
