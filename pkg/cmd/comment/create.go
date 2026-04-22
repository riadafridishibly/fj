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

type createOptions struct {
	Factory    *cmdutil.Factory
	Number     string
	Body       string
	BodyFile   string
	JSONOutput bool
}

func NewCmdCreate(f *cmdutil.Factory, k Kind) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "create <" + k.Arg + ">",
		Short: "Add a comment to " + articleA(k.Noun) + " " + k.Noun,
		Example: fmt.Sprintf(`  $ %s create 42 --body "This is a comment"
  $ %s create 42 --body-file comment.md
  $ echo "comment" | %s create 42 --body-file -`, k.CLI, k.CLI, k.CLI),
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
			return createRun(opts, k)
		},
	}

	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "The comment body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use \"-\" for stdin)")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func createRun(opts *createOptions, k Kind) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid %s number: %s", k.Noun, opts.Number)
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

// articleA picks "a" or "an" for a noun — cheap, good enough for help text.
func articleA(noun string) string {
	if noun == "" {
		return "a"
	}
	switch noun[0] {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return "an"
	}
	return "a"
}
