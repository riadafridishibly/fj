package release

import (
	"encoding/json"
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type editOptions struct {
	Factory    *cmdutil.Factory
	Tag        string
	NewTag     string
	Title      string
	Notes      string
	NotesFile  string
	Target     string
	Draft      *bool
	Prerelease *bool
	JSONOutput bool
}

func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	opts := &editOptions{Factory: f}
	var draftFlag, prereleaseFlag bool

	cmd := &cobra.Command{
		Use:   "edit <tag>",
		Short: "Edit a release",
		Example: `  $ fj release edit v1.2.0 --title "v1.2.0 - Hotfix"
  $ fj release edit v1.2.0 --notes-file CHANGELOG.md
  $ fj release edit v1.2.0 --draft=false
  $ fj release edit v1.2.0 --prerelease=true`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Tag = args[0]
			if opts.NotesFile != "" {
				body, err := cmdutil.ReadBodyFromFile(opts.NotesFile)
				if err != nil {
					return err
				}
				opts.Notes = body
			}
			if cmd.Flags().Changed("draft") {
				opts.Draft = &draftFlag
			}
			if cmd.Flags().Changed("prerelease") {
				opts.Prerelease = &prereleaseFlag
			}
			return editRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.NewTag, "tag", "", "Rename the tag")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Edit the title")
	cmd.Flags().StringVarP(&opts.Notes, "notes", "n", "", "Edit the notes")
	cmd.Flags().StringVarP(&opts.NotesFile, "notes-file", "F", "", "Read notes from file (use \"-\" for stdin)")
	cmd.Flags().StringVar(&opts.Target, "target", "", "Change target branch or commit-ish")
	cmd.Flags().BoolVarP(&draftFlag, "draft", "d", false, "Set draft state")
	cmd.Flags().BoolVarP(&prereleaseFlag, "prerelease", "p", false, "Set pre-release state")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func editRun(opts *editOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	rel, _, err := client.GetReleaseByTag(repo.Owner, repo.Name, opts.Tag)
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	form := forgejo.EditReleaseOption{
		IsDraft:      opts.Draft,
		IsPrerelease: opts.Prerelease,
	}
	if opts.NewTag != "" {
		form.TagName = opts.NewTag
	}
	if opts.Title != "" {
		form.Title = opts.Title
	}
	if opts.Notes != "" {
		form.Note = opts.Notes
	}
	if opts.Target != "" {
		form.Target = opts.Target
	}

	updated, _, err := client.EditRelease(repo.Owner, repo.Name, rel.ID, form)
	if err != nil {
		return fmt.Errorf("editing release: %w", err)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(updated)
	}

	fmt.Fprintf(os.Stderr, "✓ Edited release %s\n", updated.TagName)
	fmt.Fprintf(os.Stdout, "%s\n", updated.HTMLURL)
	return nil
}
