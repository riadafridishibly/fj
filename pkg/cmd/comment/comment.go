// Package comment provides shared implementations for issue and pull
// request comment commands. Forgejo treats PR comments as issue comments
// on the same /repos/{owner}/{repo}/issues/... endpoints, so the logic
// here is identical for both.
package comment

import (
	"fmt"

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
	// RefHelp describes the accepted reference forms in long help.
	RefHelp string
	// Resolver turns the positional argument into the repository and the
	// issue or pull request index. The pull request form accepts no
	// argument and falls back to the current branch.
	Resolver cmdutil.NumberResolver
}

// NewCmdComment returns the `comment` parent with CRUD subcommands. gh
// spells comments as a leaf verb — `gh issue comment 42 --body x` — so the
// group itself also posts a comment, dispatching to `create`.
func NewCmdComment(f *cmdutil.Factory, k Kind) *cobra.Command {
	cmd := newCreateCmd(f, k)
	// The group takes no argument when it is only naming its subcommands,
	// so the number is validated by the resolver rather than by cobra.
	cmd.Args = cmdutil.MaximumNArgs(1)
	cmd.Use = "comment " + k.Resolver.Spec + " [flags]"
	cmd.Short = "Manage " + k.Noun + " comments"
	cmd.Long = fmt.Sprintf(
		"Manage %s comments.\n\nWith a body flag the command adds a comment, exactly as %s create does.\n\n%s",
		k.Noun, k.CLI, k.RefHelp)
	cmd.Example = fmt.Sprintf(`  $ %s 42 --body "This is a comment"
  $ %s list 42
  $ %s edit 12345 --body "Updated comment"`, k.CLI, k.CLI, k.CLI)

	cmd.AddCommand(NewCmdCreate(f, k))
	cmd.AddCommand(NewCmdList(f, k))
	cmd.AddCommand(NewCmdView(f, k))
	cmd.AddCommand(NewCmdEdit(f, k))
	cmd.AddCommand(NewCmdDelete(f, k))
	return cmd
}
