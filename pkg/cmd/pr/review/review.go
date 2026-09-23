// Package review implements the `fj pr review` subcommands for creating,
// listing, and viewing pull request reviews.
package review

import (
	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/pr/review/comment"
)

func NewCmdReview(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review <command>",
		Short: "Manage pull request reviews",
	}
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(comment.NewCmdComment(f))
	return cmd
}

// reviewFields are gh's names for a review, as in gh pr view --json reviews.
// reviewFJFields are fj's additions.
var (
	reviewFields   = []string{"author", "body", "commit", "id", "state", "submittedAt"}
	reviewFJFields = []string{"official", "stale", "url"}
)

// JSON returns r keyed by gh field name, plus fj's official, stale and url.
func JSON(r *forgejo.PullReview) map[string]any {
	var login string
	if r.Reviewer != nil {
		login = r.Reviewer.UserName
	}
	return map[string]any{
		"author":      map[string]any{"login": login},
		"body":        r.Body,
		"commit":      map[string]any{"oid": r.CommitID},
		"id":          r.ID,
		"official":    r.Official,
		"stale":       r.Stale,
		"state":       ghState(r),
		"submittedAt": cmdutil.JSONTime(&r.Submitted),
		"url":         r.HTMLURL,
	}
}

// ghState is gh's name for a review's state. APPROVED and PENDING match;
// anything gh lacks, such as REQUEST_REVIEW, passes through.
func ghState(r *forgejo.PullReview) string {
	if r.Dismissed {
		return "DISMISSED"
	}
	switch r.State {
	case forgejo.ReviewStateRequestChanges:
		return "CHANGES_REQUESTED"
	case forgejo.ReviewStateComment:
		return "COMMENTED"
	}
	return string(r.State)
}
