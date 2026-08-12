package cmdutil

import (
	"fmt"
	"strconv"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/git"
)

// branchPRPageSize and branchPRMaxPages bound the search for the current
// branch's pull request. Open pull requests come back newest first, so the
// branch just pushed is near the front; the cap keeps a busy repository
// from turning one command into dozens of requests.
const (
	branchPRPageSize = 50
	branchPRMaxPages = 10
)

// NumberResolver pairs a command's argument validator with the function
// that turns those arguments into an issue or pull request index, so
// commands shared between the two differ only in this value.
type NumberResolver struct {
	Args   cobra.PositionalArgs
	Number func(f *Factory, repo Repo, args []string) (int64, error)
	// Optional reports whether the argument may be omitted, for help text.
	Optional bool
}

// ArgSpec renders the positional argument for a command's Use line.
func (r NumberResolver) ArgSpec(label string) string {
	if r.Optional {
		return "[<" + label + ">]"
	}
	return "<" + label + ">"
}

// IssueNumberResolver requires the number: issues have no per-branch form.
func IssueNumberResolver() NumberResolver {
	return NumberResolver{Args: ExactArgs(1), Number: (*Factory).IssueNumber}
}

// PRNumberResolver makes the number optional, falling back to the branch.
func PRNumberResolver() NumberResolver {
	return NumberResolver{Args: MaximumNArgs(1), Number: (*Factory).PRNumber, Optional: true}
}

// IssueNumber parses the positional argument as an issue number.
func (f *Factory) IssueNumber(_ Repo, args []string) (int64, error) {
	if len(args) == 0 {
		return 0, FlagErrorf("an issue number is required")
	}
	return parseNumber(args[0], "issue")
}

// PRNumber parses the positional argument as a pull request number. When
// the argument is omitted it resolves the open pull request for the
// currently checked-out branch, as gh does.
func (f *Factory) PRNumber(repo Repo, args []string) (int64, error) {
	if len(args) > 0 {
		return parseNumber(args[0], "pull request")
	}
	return f.currentBranchPR(repo)
}

func parseNumber(arg, noun string) (int64, error) {
	index, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || index < 1 {
		return 0, FlagErrorf("invalid %s number: %s", noun, arg)
	}
	return index, nil
}

func (f *Factory) currentBranchPR(repo Repo) (int64, error) {
	branch, err := git.CurrentBranch()
	if err != nil {
		return 0, fmt.Errorf("could not determine the current branch: %w", err)
	}

	client, err := f.ClientForRepo(repo)
	if err != nil {
		return 0, err
	}

	opt := forgejo.ListPullRequestsOptions{
		ListOptions: forgejo.ListOptions{PageSize: branchPRPageSize},
		State:       forgejo.StateOpen,
	}

	var all []*forgejo.PullRequest
	for page := 1; page <= branchPRMaxPages; page++ {
		opt.Page = page
		prs, _, err := client.ListRepoPullRequests(repo.Owner, repo.Name, opt)
		if err != nil {
			return 0, fmt.Errorf("listing pull requests: %w", err)
		}
		all = append(all, prs...)
		if len(prs) < branchPRPageSize {
			break
		}
	}

	return pickBranchPR(all, repo, branch)
}

// pickBranchPR chooses the pull request whose head is branch. Pull requests
// opened from the base repository itself win over same-named branches on
// forks, which are somebody else's work.
func pickBranchPR(prs []*forgejo.PullRequest, repo Repo, branch string) (int64, error) {
	var local, foreign []int64
	for _, pr := range prs {
		if pr.Head == nil || pr.Head.Ref != branch {
			continue
		}
		if pr.Head.Repository != nil && !strings.EqualFold(pr.Head.Repository.FullName, repo.FullName()) {
			foreign = append(foreign, pr.Index)
		} else {
			local = append(local, pr.Index)
		}
	}

	matches := local
	if len(matches) == 0 {
		matches = foreign
	}
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no open pull request found for branch %q in %s", branch, repo.FullName())
	case 1:
		return matches[0], nil
	}
	return 0, fmt.Errorf("branch %q has %d open pull requests (%s); specify one by number",
		branch, len(matches), joinNumbers(matches))
}

func joinNumbers(numbers []int64) string {
	parts := make([]string, len(numbers))
	for i, n := range numbers {
		parts[i] = "#" + strconv.FormatInt(n, 10)
	}
	return strings.Join(parts, ", ")
}
