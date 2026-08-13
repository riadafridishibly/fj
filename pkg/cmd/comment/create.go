package comment

import (
	"fmt"
	"os"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type createOptions struct {
	Factory    *cmdutil.Factory
	Args       []string
	Body       string
	BodyFile   string
	JSONOutput bool
}

func NewCmdCreate(f *cmdutil.Factory, k Kind) *cobra.Command {
	cmd := newCreateCmd(f, k)
	cmd.Use = "create " + k.Resolver.ArgSpec(k.Arg)
	cmd.Short = "Add a comment to " + articleA(k.Noun) + " " + k.Noun
	cmd.Example = k.example(`  $ %s create 42 --body "This is a comment"
  $ %s create 42 --body-file comment.md
  $ echo "comment" | %s create 42 --body-file -`, `  $ %s create --body "This is a comment"`)
	return cmd
}

// newCreateCmd builds the command that posts a comment. It backs both the
// `create` subcommand and the `comment` group itself, so the two cannot
// drift apart; callers set Use, Short, and Example for their spelling.
func newCreateCmd(f *cmdutil.Factory, k Kind) *cobra.Command {
	opts := &createOptions{Factory: f}

	cmd := &cobra.Command{
		Args: k.Resolver.Args(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmdutil.IsBareGroupInvocation(cmd, args) {
				return cmd.Help()
			}
			// The group mounts this with the laxer GroupDispatchArgs, which
			// accepts zero arguments so `fj pr comment --body x` works; the
			// resolver's own validator is what rejects the same spelling for
			// issues, with usage.
			if err := k.Resolver.Args()(cmd, args); err != nil {
				return err
			}
			opts.Args = args
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

	index, repo, err := k.Resolver.Number(opts.Factory, repo, opts.Args)
	if err != nil {
		return err
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
		return output.PrintJSON(os.Stdout, comment)
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
