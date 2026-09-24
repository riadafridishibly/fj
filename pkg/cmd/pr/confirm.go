package pr

import (
	"fmt"
	"os"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// confirmedCurrentBranchPR resolves the pull request for the checked-out
// branch on behalf of a destructive command. It reports what was resolved
// and refuses to continue unless the user passed --yes, so a forgotten
// number cannot merge or close the wrong pull request.
func confirmedCurrentBranchPR(f *cmdutil.Factory, repo cmdutil.Repo, yes bool, action string) (int64, cmdutil.Repo, error) {
	index, repo, err := f.PRNumber(repo, nil)
	if err != nil {
		return 0, repo, err
	}

	client, err := f.ClientForRepo(repo)
	if err != nil {
		return 0, repo, err
	}
	pr, _, err := client.GetPullRequest(repo.Owner, repo.Name, index)
	if err != nil {
		return 0, repo, fmt.Errorf("getting pull request: %w", err)
	}

	head := ""
	if pr.Head != nil {
		head = pr.Head.Ref
	}
	fmt.Fprintf(os.Stderr, "Resolved pull request #%d (%s) from branch %q\n", index, pr.Title, head)

	if !yes {
		return 0, repo, cmdutil.FlagErrorf(
			"re-run with --yes to %s #%d, or pass the pull request number explicitly", action, index)
	}
	return index, repo, nil
}
