// Package comment provides shared implementations for issue and pull
// request comment commands. Forgejo treats PR comments as issue comments
// on the same /repos/{owner}/{repo}/issues/... endpoints, so the logic
// here is identical for both.
package comment

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// Kind customizes help text so the same subcommands can be mounted under
// both `fj issue comment` and `fj pr comment`.
type Kind struct {
	// Noun is the human-readable subject, e.g. "issue" or "pull request".
	Noun string
	// CLI is the command prefix shown in examples,
	// e.g. "fj issue comment" or "fj pr comment".
	CLI string
	// Arg is the positional-arg label for create/list, e.g. "issue" or "pr".
	Arg string
}

// NewCmdComment returns the `comment` parent with CRUD subcommands.
func NewCmdComment(f *cmdutil.Factory, k Kind) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <command>",
		Short: "Manage " + k.Noun + " comments",
	}
	cmd.AddCommand(NewCmdCreate(f, k))
	cmd.AddCommand(NewCmdList(f, k))
	cmd.AddCommand(NewCmdView(f, k))
	cmd.AddCommand(NewCmdEdit(f, k))
	cmd.AddCommand(NewCmdDelete(f, k))
	return cmd
}
