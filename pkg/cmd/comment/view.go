package comment

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type viewOptions struct {
	Factory    *cmdutil.Factory
	ID         string
	Web        bool
	JSONOutput bool
}

func NewCmdView(f *cmdutil.Factory, k Kind) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view <id>",
		Short: "View a comment by its ID",
		Example: fmt.Sprintf(`  $ %s view 12345
  $ %s view 12345 --web
  $ %s view 12345 --json`, k.CLI, k.CLI, k.CLI),
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			return viewRun(opts)
		},
	}

	cmdutil.AddWebFlag(cmd, &opts.Web)
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func viewRun(opts *viewOptions) error {
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

	comment, _, err := client.GetIssueComment(repo.Owner, repo.Name, id)
	if err != nil {
		return fmt.Errorf("getting comment: %w", err)
	}

	if opts.Web {
		return cmdutil.OpenInBrowser(comment.HTMLURL)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(comment)
	}

	author := "unknown"
	if comment.Poster != nil {
		author = comment.Poster.UserName
	}
	fmt.Fprintf(os.Stdout, "Comment #%d by %s\n", comment.ID, author)
	fmt.Fprintf(os.Stdout, "Created: %s\n", output.RelativeTimeStr(comment.Created))
	if !comment.Updated.Equal(comment.Created) {
		fmt.Fprintf(os.Stdout, "Updated: %s\n", output.RelativeTimeStr(comment.Updated))
	}
	if comment.Body != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", output.RenderMarkdown(comment.Body))
	}
	fmt.Fprintf(os.Stdout, "\nView on the web: %s\n", comment.HTMLURL)
	return nil
}
