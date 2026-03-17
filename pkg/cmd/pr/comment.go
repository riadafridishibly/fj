package pr

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type commentOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	Body       string
	BodyFile   string
	JSONOutput bool
}

func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	opts := &commentOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "comment <number>",
		Short: "Add a comment to a pull request",
		Example: `  $ fj pr comment 42 --body "Looks good to me"
  $ fj pr comment 42 --body-file review.md
  $ echo "LGTM" | fj pr comment 42 --body-file -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
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
			return commentRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "The comment body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func commentRun(opts *commentOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid pull request number: %s", opts.Number)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	comment, _, err := client.CreateIssueComment(repo.Owner, repo.Name, index, forgejo.CreateIssueCommentOption{
		Body: opts.Body,
	})
	if err != nil {
		return fmt.Errorf("adding comment: %w", err)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(comment)
	}

	fmt.Fprintf(os.Stdout, "%s\n", comment.HTMLURL)
	return nil
}
