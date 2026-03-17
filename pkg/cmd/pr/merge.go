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

type mergeOptions struct {
	Factory      *cmdutil.Factory
	Number       string
	Method       string
	Title        string
	Message      string
	DeleteBranch bool
	Auto         bool
	JSONOutput   bool
}

func NewCmdMerge(f *cmdutil.Factory) *cobra.Command {
	opts := &mergeOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "merge <number>",
		Short: "Merge a pull request",
		Example: `  $ fj pr merge 42
  $ fj pr merge 42 --squash
  $ fj pr merge 42 --rebase
  $ fj pr merge 42 --delete-branch
  $ fj pr merge 42 --merge --title "Merge feature" --message "Detailed description"
  $ fj pr merge 42 --auto`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]

			// Determine merge method from flags
			squash, _ := cmd.Flags().GetBool("squash")
			rebase, _ := cmd.Flags().GetBool("rebase")
			merge, _ := cmd.Flags().GetBool("merge")

			methods := 0
			if squash {
				methods++
				opts.Method = "squash"
			}
			if rebase {
				methods++
				opts.Method = "rebase"
			}
			if merge {
				methods++
				opts.Method = "merge"
			}
			if methods > 1 {
				return cmdutil.FlagErrorf("specify only one of --merge, --squash, --rebase")
			}
			if methods == 0 {
				opts.Method = "merge"
			}

			return mergeRun(opts)
		},
	}

	cmd.Flags().Bool("merge", false, "Merge the commits (default)")
	cmd.Flags().Bool("squash", false, "Squash and merge")
	cmd.Flags().Bool("rebase", false, "Rebase and merge")
	cmd.Flags().StringVar(&opts.Title, "title", "", "Title for the merge commit")
	cmd.Flags().StringVar(&opts.Message, "message", "", "Message for the merge commit")
	cmd.Flags().BoolVarP(&opts.DeleteBranch, "delete-branch", "d", false, "Delete the branch after merge")
	cmd.Flags().BoolVar(&opts.Auto, "auto", false, "Merge when all checks pass")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func mergeRun(opts *mergeOptions) error {
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

	mergeOpt := forgejo.MergePullRequestOption{
		Style:                  forgejo.MergeStyle(opts.Method),
		Title:                  opts.Title,
		Message:                opts.Message,
		DeleteBranchAfterMerge: opts.DeleteBranch,
		MergeWhenChecksSucceed: opts.Auto,
	}

	merged, _, err := client.MergePullRequest(repo.Owner, repo.Name, index, mergeOpt)
	if err != nil {
		return fmt.Errorf("merging pull request: %w", err)
	}

	if opts.JSONOutput {
		pr, _, err := client.GetPullRequest(repo.Owner, repo.Name, index)
		if err != nil {
			return fmt.Errorf("getting merged pull request: %w", err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pr)
	}

	if opts.Auto {
		fmt.Fprintf(os.Stderr, "✓ Pull request #%d will be merged automatically when all checks pass\n", index)
	} else if merged {
		fmt.Fprintf(os.Stderr, "✓ Merged pull request #%d\n", index)
	} else {
		fmt.Fprintf(os.Stderr, "Pull request #%d was not merged\n", index)
	}

	return nil
}
