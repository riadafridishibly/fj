package comment

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type deleteOptions struct {
	Factory *cmdutil.Factory
	ID      string
	Yes     bool
	DryRun  bool
}

func NewCmdDelete(f *cmdutil.Factory, k Kind) *cobra.Command {
	opts := &deleteOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a comment by its ID",
		Example: fmt.Sprintf(`  $ %s delete 12345 --yes
  $ %s delete 12345 --dry-run`, k.CLI, k.CLI),
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			return deleteRun(opts)
		},
	}

	cmdutil.AddDeleteFlags(cmd, &opts.Yes, &opts.DryRun)

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	perform, err := cmdutil.ResolveDeleteFlags(opts.Yes, opts.DryRun)
	if err != nil {
		return err
	}

	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	id, err := strconv.ParseInt(opts.ID, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid comment id: %s", opts.ID)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	// Fetch the comment first so a wrong id fails before anything is deleted,
	// and so the user can verify the target from author + body excerpt.
	comment, _, err := client.GetIssueComment(repo.Owner, repo.Name, id)
	if err != nil {
		return fmt.Errorf("fetching comment #%d: %w", id, err)
	}

	author := "unknown"
	if comment.Poster != nil {
		author = comment.Poster.UserName
	}
	fmt.Fprintf(os.Stderr, "Comment #%d by %s:\n  %s\n", id, author, excerpt(comment.Body))

	if !perform {
		fmt.Fprintln(os.Stderr, "(dry-run; no changes were made)")
		return nil
	}

	if _, err := client.DeleteIssueComment(repo.Owner, repo.Name, id); err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Deleted comment #%d\n", id)
	return nil
}

// excerpt renders the first line of a comment body, truncated for display.
func excerpt(body string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	if len(line) > 80 {
		line = line[:77] + "..."
	}
	return line
}
