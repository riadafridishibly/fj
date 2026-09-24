package pr

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/git"
)

type checkoutOptions struct {
	Factory *cmdutil.Factory
	Args    []string
}

func NewCmdCheckout(f *cmdutil.Factory) *cobra.Command {
	opts := &checkoutOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "checkout <number>",
		Short:   "Check out a pull request locally",
		Long:    "Check out a pull request locally.",
		Example: `  $ fj pr checkout 42`,
		Args:    cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return checkoutRun(opts)
		},
	}

	return cmd
}

func checkoutRun(opts *checkoutOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, repo, err := opts.Factory.RequiredPRNumber(repo, opts.Args)
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	pr, _, err := client.GetPullRequest(repo.Owner, repo.Name, index)
	if err != nil {
		return fmt.Errorf("getting pull request: %w", err)
	}

	if pr.Head == nil {
		return fmt.Errorf("pull request #%d has no head branch info", index)
	}

	branchName := pr.Head.Ref
	if current, err := git.CurrentBranch(); err == nil && current == branchName {
		fmt.Fprintf(os.Stderr, "Branch '%s' is already checked out\n", branchName)
		return nil
	}

	// Fetch the PR branch
	refSpec := fmt.Sprintf("pull/%d/head:%s", index, branchName)
	if err := git.Fetch("origin", refSpec); err != nil {
		return fmt.Errorf("fetching PR branch: %w", err)
	}

	if err := git.Checkout(branchName); err != nil {
		return fmt.Errorf("checking out branch: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Switched to branch '%s'\n", branchName)
	return nil
}
