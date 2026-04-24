package comment

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type editOptions struct {
	Factory    *cmdutil.Factory
	ID         string
	Body       string
	BodyFile   string
	JSONOutput bool
}

func NewCmdEdit(f *cmdutil.Factory, k Kind) *cobra.Command {
	opts := &editOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a comment by its ID",
		Example: fmt.Sprintf(`  $ %s edit 12345 --body "Updated comment"
  $ %s edit 12345 --body-file updated.md
  $ echo "new body" | %s edit 12345 --body-file -`, k.CLI, k.CLI, k.CLI),
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if opts.BodyFile != "" {
				body, err := cmdutil.ReadBodyFromFile(opts.BodyFile)
				if err != nil {
					return err
				}
				opts.Body = body
			}
			if opts.Body == "" {
				return cmdutil.FlagErrorf("--body or --body-file is required")
			}
			return editRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "The new comment body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func editRun(opts *editOptions) error {
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

	comment, _, err := client.EditIssueComment(repo.Owner, repo.Name, id, forgejo.EditIssueCommentOption{
		Body: opts.Body,
	})
	if err != nil {
		return fmt.Errorf("editing comment: %w", err)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(comment)
	}

	fmt.Fprintf(os.Stdout, "%s\n", comment.HTMLURL)
	return nil
}
