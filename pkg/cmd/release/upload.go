package release

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type uploadOptions struct {
	Factory *cmdutil.Factory
	Tag     string
	Files   []string
	Clobber bool
}

func NewCmdUpload(f *cmdutil.Factory) *cobra.Command {
	opts := &uploadOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "upload <tag> <file>...",
		Short: "Upload assets to a release",
		Example: `  $ fj release upload v1.2.0 dist/*.tar.gz
  $ fj release upload v1.2.0 binary --clobber`,
		Args: cmdutil.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Tag = args[0]
			opts.Files = args[1:]
			return uploadRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Clobber, "clobber", false, "Overwrite existing assets with the same name")

	return cmd
}

func uploadRun(opts *uploadOptions) error {
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

	existing := make(map[string]int64)
	for _, a := range rel.Attachments {
		existing[a.Name] = a.ID
	}

	for _, path := range opts.Files {
		name := filepath.Base(path)
		if id, ok := existing[name]; ok {
			if !opts.Clobber {
				return fmt.Errorf("asset %q already exists (use --clobber to overwrite)", name)
			}
			if _, err := client.DeleteReleaseAttachment(repo.Owner, repo.Name, rel.ID, id); err != nil {
				return fmt.Errorf("deleting existing asset %q: %w", name, err)
			}
		}

		if err := uploadAsset(client, repo, rel.ID, path); err != nil {
			return fmt.Errorf("uploading %s: %w", path, err)
		}
		fmt.Fprintf(os.Stderr, "✓ Uploaded %s\n", name)
	}

	return nil
}
