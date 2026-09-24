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

// NumberResolver describes how a command obtains its issue or pull request
// index, so commands shared between the two differ only in this value.
type NumberResolver struct {
	// Optional marks the positional number as omittable; the argument
	// validator and the help text both derive from it.
	Optional bool
	Number   func(f *Factory, repo Repo, args []string) (int64, Repo, error)
}

// Args returns the positional-argument validator implied by Optional.
func (r NumberResolver) Args() cobra.PositionalArgs {
	if r.Optional {
		return MaximumNArgs(1)
	}
	return ExactArgs(1)
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
	return NumberResolver{Number: (*Factory).IssueNumber}
}

// PRNumberResolver makes the number optional, falling back to the branch.
func PRNumberResolver() NumberResolver {
	return NumberResolver{Optional: true, Number: (*Factory).PRNumber}
}

// IssueNumber parses the positional argument as an issue number.
func (f *Factory) IssueNumber(repo Repo, args []string) (int64, Repo, error) {
	if len(args) == 0 {
		return 0, repo, FlagErrorf("an issue number is required")
	}
	index, err := ParseNumber(args[0], "issue number")
	return index, repo, err
}

// RequiredPRNumber parses the positional argument as a pull request number
// with no branch fallback, for commands like checkout where the current
// branch's pull request cannot be the answer.
func (f *Factory) RequiredPRNumber(repo Repo, args []string) (int64, Repo, error) {
	if len(args) == 0 {
		return 0, repo, FlagErrorf("a pull request number is required")
	}
	index, err := ParseNumber(args[0], "pull request number")
	return index, repo, err
}

// PRNumber parses the positional argument as a pull request number. When
// the argument is omitted it resolves the open pull request for the
// currently checked-out branch, as gh does. The returned Repo is where the
// pull request lives: normally the input repo, but its parent when the
// branch's pull request was opened from a fork against upstream.
func (f *Factory) PRNumber(repo Repo, args []string) (int64, Repo, error) {
	if len(args) > 0 {
		index, err := ParseNumber(args[0], "pull request number")
		return index, repo, err
	}
	// The checkout's branch says nothing about another repository.
	if f.RepoOverride != "" {
		return 0, repo, FlagErrorf("argument required when using the --repo flag")
	}
	return f.currentBranchPR(repo)
}

// ParseNumber parses a positional index. Anything that is not a positive
// integer is a usage error, so a typo exits 2 instead of reaching the API and
// coming back as a 404. label names the value in the message, for example
// "issue number" or "review id".
func ParseNumber(arg, label string) (int64, error) {
	index, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || index < 1 {
		return 0, FlagErrorf("invalid %s: %s", label, arg)
	}
	return index, nil
}

func (f *Factory) currentBranchPR(repo Repo) (int64, Repo, error) {
	branch, err := git.CurrentBranch()
	if err != nil {
		return 0, repo, FlagErrorf("not on any branch; specify a pull request number")
	}

	client, err := f.ClientForRepo(repo)
	if err != nil {
		return 0, repo, err
	}

	return branchPR(client, repo, branch)
}

// branchPR resolves branch's open pull request, looking first in repo and
// then, when repo is a fork, in the repository it was forked from.
func branchPR(client *forgejo.Client, repo Repo, branch string) (int64, Repo, error) {
	index, found, err := findBranchPR(client, repo, repo, branch)
	if err != nil {
		return 0, repo, err
	}
	if found {
		return index, repo, nil
	}

	parent, err := parentRepo(client, repo)
	if err != nil {
		return 0, repo, err
	}
	if parent == nil {
		return 0, repo, noBranchPRError(branch, repo.FullName())
	}

	index, found, err = findBranchPR(client, *parent, repo, branch)
	if err != nil {
		return 0, repo, err
	}
	if !found {
		return 0, repo, noBranchPRError(branch, repo.FullName()+" or "+parent.FullName())
	}
	return index, *parent, nil
}

// noBranchPRError has a stable, greppable prefix; change the wording only
// deliberately.
func noBranchPRError(branch, searched string) error {
	return fmt.Errorf("no open pull request found for branch %q in %s", branch, searched)
}

// parentRepo returns the repository repo was forked from, or nil when it is
// not a fork.
func parentRepo(client *forgejo.Client, repo Repo) (*Repo, error) {
	r, _, err := client.GetRepo(repo.Owner, repo.Name)
	if err != nil {
		return nil, fmt.Errorf("getting repository %s: %w", repo.FullName(), err)
	}
	if r.Parent == nil || r.Parent.Owner == nil {
		return nil, nil
	}
	return &Repo{Host: repo.Host, Owner: r.Parent.Owner.UserName, Name: r.Parent.Name}, nil
}

// findBranchPR searches base's open pull requests for the one whose head is
// branch in head's repository. found is false when there is none.
func findBranchPR(client *forgejo.Client, base, head Repo, branch string) (int64, bool, error) {
	opt := forgejo.ListPullRequestsOptions{
		ListOptions: forgejo.ListOptions{PageSize: branchPRPageSize},
		State:       forgejo.StateOpen,
	}

	var matches branchMatches
	seen := 0
	for page := 1; page <= branchPRMaxPages; page++ {
		opt.Page = page
		prs, resp, err := client.ListRepoPullRequests(base.Owner, base.Name, opt)
		if err != nil {
			return 0, false, fmt.Errorf("listing pull requests: %w", err)
		}
		matches.collect(prs, head, branch)
		seen += len(prs)
		// The server may clamp the page size below what was asked for, so a
		// short page does not mean the last page; trust the reported total
		// or an empty page instead.
		if total := TotalCount(resp); len(prs) == 0 || (total > 0 && seen >= total) {
			break
		}
	}
	return matches.pick(branch)
}

// branchMatches collects the open pull requests whose head is the branch in
// the head repository. A same-named branch on another fork is somebody
// else's work, and a head whose fork was deleted cannot be traced to anyone,
// so neither counts.
// ponytail: only the checkout's own repository counts as ours, so a branch
// pushed to a second fork remote is not found; read branch.<name>.pushRemote
// if that workflow matters.
type branchMatches []int64

func (m *branchMatches) collect(prs []*forgejo.PullRequest, head Repo, branch string) {
	for _, pr := range prs {
		if pr.Head != nil && pr.Head.Ref == branch && pr.Head.Repository != nil &&
			strings.EqualFold(pr.Head.Repository.FullName, head.FullName()) {
			*m = append(*m, pr.Index)
		}
	}
}

// pick chooses the match; two or more are an error the user must break by
// passing a number.
func (m branchMatches) pick(branch string) (int64, bool, error) {
	switch len(m) {
	case 0:
		return 0, false, nil
	case 1:
		return m[0], true, nil
	}
	return 0, false, fmt.Errorf("branch %q has %d open pull requests (%s); specify one by number",
		branch, len(m), joinNumbers(m))
}

func joinNumbers(numbers []int64) string {
	parts := make([]string, len(numbers))
	for i, n := range numbers {
		parts[i] = "#" + strconv.FormatInt(n, 10)
	}
	return strings.Join(parts, ", ")
}
