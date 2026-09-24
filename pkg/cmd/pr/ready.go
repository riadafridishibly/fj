package pr

import (
	"fmt"
	"os"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// wipPrefixes are Forgejo's default WORK_IN_PROGRESS_PREFIXES. A server can
// set others, so readyRun checks the draft state again after the edit.
var wipPrefixes = []string{"WIP:", "[WIP]"}

// draftPrefix is what pr create --draft and pr ready --undo put before a
// title to make the pull request a draft.
const draftPrefix = "WIP: "

type readyOptions struct {
	Factory *cmdutil.Factory
	Args    []string
	Undo    bool
}

func NewCmdReady(f *cmdutil.Factory) *cobra.Command {
	opts := &readyOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "ready [<number>]",
		Short: "Mark a pull request as ready for review",
		Long: `Mark a pull request as ready for review, or convert it to a draft with --undo.

Forgejo has no separate draft flag: a pull request is a draft while its title
starts with a work-in-progress prefix, WIP: or [WIP] by default. This command
removes the prefix, or with --undo adds WIP: to the title.

With no number, the pull request for the current branch is used.`,
		Example: `  $ fj pr ready 42
  $ fj pr ready 42 --undo
  $ fj pr ready`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return readyRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Undo, "undo", false, `Convert a pull request to "draft"`)

	return cmd
}

func readyRun(opts *readyOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, repo, err := opts.Factory.PRNumber(repo, opts.Args)
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	pr, err := getPR(opts.Factory, repo, index)
	if err != nil {
		return err
	}
	if pr.State != forgejo.StateOpen {
		return fmt.Errorf("pull request %s#%d is closed. Only draft pull requests can be marked as \"ready for review\"", repo.FullName(), index)
	}

	want := opts.Undo
	if pr.Draft == want {
		fmt.Fprintf(os.Stderr, "! Pull request %s#%d is already %s\n", repo.FullName(), index, draftState(want))
		return nil
	}

	title := draftPrefix + pr.Title
	if !want {
		title = stripWIP(pr.Title)
		switch title {
		case pr.Title:
			return fmt.Errorf("pull request %s#%d is a draft, but its title %q has no WIP: or [WIP] prefix to remove; the server sets its own WORK_IN_PROGRESS_PREFIXES, so change the title with fj pr edit", repo.FullName(), index, pr.Title)
		case "":
			return fmt.Errorf("pull request %s#%d has no title besides %q; set one with fj pr edit %d --title", repo.FullName(), index, pr.Title, index)
		}
	}

	if _, _, err := client.EditPullRequest(repo.Owner, repo.Name, index, forgejo.EditPullRequestOption{Title: title}); err != nil {
		return fmt.Errorf("editing pull request title: %w", err)
	}

	after, err := getPR(opts.Factory, repo, index)
	if err != nil {
		return err
	}
	if after.Draft != want {
		if _, _, err := client.EditPullRequest(repo.Owner, repo.Name, index, forgejo.EditPullRequestOption{Title: pr.Title}); err != nil {
			return fmt.Errorf("forgejo did not treat the title %q as %s, and restoring the title %q failed: %w", title, draftState(want), pr.Title, err)
		}
		return fmt.Errorf("forgejo did not treat the title %q as %s, so the title was restored; the server sets its own WORK_IN_PROGRESS_PREFIXES, so change the title with fj pr edit", title, draftState(want))
	}

	if want {
		fmt.Fprintf(os.Stderr, "✓ Pull request %s#%d is converted to \"draft\"\n", repo.FullName(), index)
	} else {
		fmt.Fprintf(os.Stderr, "✓ Pull request %s#%d is marked as \"ready for review\"\n", repo.FullName(), index)
	}
	return nil
}

func draftState(draft bool) string {
	if draft {
		return `"in draft"`
	}
	return `"ready for review"`
}

// stripWIP removes every leading work-in-progress prefix, ignoring case as
// Forgejo does.
func stripWIP(title string) string {
	for _, p := range wipPrefixes {
		if len(title) >= len(p) && strings.EqualFold(title[:len(p)], p) {
			return stripWIP(strings.TrimSpace(title[len(p):]))
		}
	}
	return title
}
