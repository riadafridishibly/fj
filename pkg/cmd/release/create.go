package release

import (
	"fmt"
	"os"
	"path/filepath"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type createOptions struct {
	Factory    *cmdutil.Factory
	Tag        string
	Title      string
	Notes      string
	NotesFile  string
	Target     string
	Draft      bool
	Prerelease bool
	Assets     []string
	JSONOutput bool
}

func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "create <tag> [<file>...]",
		Short:   "Create a new release",
		Aliases: []string{"new"},
		Example: `  $ fj release create v1.2.0 --title "v1.2.0" --notes "Release notes"
  $ fj release create v1.2.0 --notes-file CHANGELOG.md
  $ fj release create v1.2.0 --draft
  $ fj release create v1.2.0 --prerelease --target main
  $ fj release create v1.2.0 dist/fj_linux_amd64.tar.gz dist/fj_darwin_amd64.tar.gz
  $ fj release create v1.2.0 --notes-file - < CHANGELOG.md`,
		Args: cmdutil.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Tag = args[0]
			opts.Assets = args[1:]
			if opts.NotesFile != "" {
				body, err := cmdutil.ReadBodyFromFile(opts.NotesFile)
				if err != nil {
					return err
				}
				opts.Notes = body
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Release title (defaults to tag)")
	cmd.Flags().StringVarP(&opts.Notes, "notes", "n", "", "Release notes")
	cmd.Flags().StringVarP(&opts.NotesFile, "notes-file", "F", "", "Read notes from file (use \"-\" for stdin)")
	cmd.Flags().StringVar(&opts.Target, "target", "", "Target branch or commit-ish (defaults to default branch)")
	cmd.Flags().BoolVarP(&opts.Draft, "draft", "d", false, "Save as draft (unpublished)")
	cmd.Flags().BoolVarP(&opts.Prerelease, "prerelease", "p", false, "Mark as pre-release")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func createRun(opts *createOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	title := opts.Title
	if title == "" {
		title = opts.Tag
	}

	createOpt := forgejo.CreateReleaseOption{
		TagName:      opts.Tag,
		Target:       opts.Target,
		Title:        title,
		Note:         opts.Notes,
		IsDraft:      opts.Draft,
		IsPrerelease: opts.Prerelease,
	}

	rel, _, err := client.CreateRelease(repo.Owner, repo.Name, createOpt)
	if err != nil {
		return fmt.Errorf("creating release: %w", err)
	}

	for _, path := range opts.Assets {
		if err := uploadAsset(client, repo, rel.ID, path); err != nil {
			return fmt.Errorf("uploading %s: %w", path, err)
		}
		fmt.Fprintf(os.Stderr, "✓ Uploaded %s\n", filepath.Base(path))
	}

	if opts.JSONOutput {
		// Re-fetch to include uploaded attachments
		rel, _, err = client.GetRelease(repo.Owner, repo.Name, rel.ID)
		if err != nil {
			return fmt.Errorf("getting release: %w", err)
		}
		return output.PrintJSON(os.Stdout, rel)
	}

	fmt.Fprintf(os.Stdout, "%s\n", rel.HTMLURL)
	return nil
}

func uploadAsset(client *forgejo.Client, repo cmdutil.Repo, releaseID int64, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _, err = client.CreateReleaseAttachment(repo.Owner, repo.Name, releaseID, file, filepath.Base(path))
	return err
}
