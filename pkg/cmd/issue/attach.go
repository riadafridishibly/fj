package issue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type attachOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	Files      []string
	JSONOutput bool
}

func NewCmdAttach(f *cmdutil.Factory) *cobra.Command {
	opts := &attachOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "attach <number> <file>...",
		Short: "Attach files to an issue",
		Long: `Upload files to an issue and print the URL of each one.

Files upload in the order given, and their URLs go to stdout, one per line.
Paste a URL into an issue body or a comment to show the file there, as in
![screenshot](URL).

Every path is checked before the first upload starts, so a typo fails the
command instead of leaving half the files attached.`,
		Example: `  $ fj issue attach 42 screenshot.png
  $ fj issue attach 42 before.png after.png
  $ fj issue attach 42 crash.log --json`,
		Args: cmdutil.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			opts.Files = args[1:]
			return attachRun(opts)
		},
	}

	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func attachRun(opts *attachOptions) error {
	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid issue number: %s", opts.Number)
	}

	if err := checkAttachableFiles(opts.Files); err != nil {
		return err
	}

	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	client, err := opts.Factory.APIClient(repo)
	if err != nil {
		return err
	}

	// Report each upload as it lands rather than at the end: an upload that
	// fails halfway still leaves the user holding the URLs it did get.
	attachments := make([]*forgejo.Attachment, 0, len(opts.Files))
	var uploadErr error
	for _, path := range opts.Files {
		att, err := attachFile(client, repo, index, path)
		if err != nil {
			uploadErr = fmt.Errorf("uploading %s: %w", path, err)
			break
		}
		attachments = append(attachments, att)
		if !opts.JSONOutput {
			fmt.Fprintf(os.Stderr, "✓ Uploaded %s\n", filepath.Base(path))
			fmt.Fprintf(os.Stdout, "%s\n", att.DownloadURL)
		}
	}

	if opts.JSONOutput && len(attachments) > 0 {
		if err := output.PrintJSON(os.Stdout, attachments); err != nil {
			return err
		}
	}

	if uploadErr != nil {
		if len(attachments) > 0 {
			fmt.Fprintf(os.Stderr,
				"%d file(s) uploaded before the failure are still attached to issue #%d\n",
				len(attachments), index)
		}
		return uploadErr
	}

	return nil
}

// checkAttachableFiles verifies every path before a single byte is uploaded.
// A typo in the last argument should not be discovered only after the files
// ahead of it have already landed on the issue.
func checkAttachableFiles(paths []string) error {
	for _, path := range paths {
		// os.Stat and os.Open name the path in their own errors, so there
		// is nothing useful to add by wrapping them.
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory, not a file", path)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

// attachFile uploads one file to an issue. Forgejo stores it under the
// path's base name, so attaching ./shots/login.png shows up as login.png.
func attachFile(client *api.Client, repo cmdutil.Repo, index int64, path string) (*forgejo.Attachment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return client.CreateIssueAttachment(repo.Owner, repo.Name, index, f, filepath.Base(path))
}
