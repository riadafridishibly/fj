package cmdutil

import (
	"fmt"
	"strconv"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/git"
)

// branchPRPageSize and branchPRMaxPages bound the search for a branch's
// pull request. Open pull requests come back newest first, so the branch
// just pushed is near the front; the cap keeps a busy repository from
// turning one command into dozens of requests.
const (
	branchPRPageSize = 50
	branchPRMaxPages = 10
)

// Argument specs for Use lines, spelled exactly as gh spells them so that
// `fj help` and `gh help` describe the same shapes for the same command.
const (
	IssueRefSpec = "{<number> | <url>}"
	PRRefSpec    = "[<number> | <url> | <branch>]"
)

// Reference help for command long text. gh documents the URL form there
// rather than in the usage line; OWNER/REPO#42 is fj's addition on top.
const (
	IssueRefHelp = "The issue may be given as a number, an issue URL, or OWNER/REPO#42."
	PRRefHelp    = "The pull request may be given as a number, a pull request URL, a branch " +
		"name, or OWNER/REPO#42. With no argument, the pull request for the current branch is used."
)

// NumberResolver pairs a command's argument validator with the function
// that turns those arguments into a repository and an issue or pull
// request index, so commands shared between the two differ only in this
// value.
type NumberResolver struct {
	Args   cobra.PositionalArgs
	Number func(f *Factory, args []string) (Repo, int64, error)
	// Spec is the positional argument as it appears in the Use line.
	Spec string
}

// IssueNumberResolver requires the reference: issues have no branch form.
func IssueNumberResolver() NumberResolver {
	return NumberResolver{Args: ExactArgs(1), Number: (*Factory).IssueNumber, Spec: IssueRefSpec}
}

// PRNumberResolver makes the reference optional, falling back to the branch.
func PRNumberResolver() NumberResolver {
	return NumberResolver{Args: MaximumNArgs(1), Number: (*Factory).PRNumber, Spec: PRRefSpec}
}

// IssueNumber resolves the positional argument to an issue and the
// repository holding it. A reference that names its own repository — an
// issue URL, or OWNER/REPO#42 — resolves outside a git checkout, so the
// base repository is looked up only when the reference omits one.
func (f *Factory) IssueNumber(args []string) (Repo, int64, error) {
	if len(args) == 0 {
		return Repo{}, 0, FlagErrorf("an issue number is required")
	}
	ref, err := ParseRef(args[0], IssueRefKind)
	if err != nil {
		return Repo{}, 0, err
	}
	if ref.Branch != "" {
		return Repo{}, 0, FlagErrorf(
			"invalid reference %q: issues have no branch form, expected a number, "+
				"an issue URL, or OWNER/REPO#42", ref.Branch)
	}
	repo, err := f.refRepo(ref)
	if err != nil {
		return Repo{}, 0, err
	}
	return repo, ref.Number, nil
}

// PRNumber resolves the positional argument to a pull request and the
// repository holding it, accepting the same references as IssueNumber
// plus a branch name. When the argument is omitted it resolves the open
// pull request for the currently checked-out branch, as gh does.
func (f *Factory) PRNumber(args []string) (Repo, int64, error) {
	if len(args) == 0 {
		repo, err := f.BaseRepo()
		if err != nil {
			return Repo{}, 0, err
		}
		branch, err := git.CurrentBranch()
		if err != nil {
			return Repo{}, 0, fmt.Errorf("could not determine the current branch: %w", err)
		}
		return f.branchPR(repo, branch)
	}

	ref, err := ParseRef(args[0], PRRefKind)
	if err != nil {
		return Repo{}, 0, err
	}
	repo, err := f.refRepo(ref)
	if err != nil {
		return Repo{}, 0, err
	}
	if ref.Branch != "" {
		return f.branchPR(repo, ref.Branch)
	}
	return repo, ref.Number, nil
}

// refRepo resolves the repository a reference belongs to: the one it
// names, or the base repository when it names none. A reference that
// disagrees with -R is a mistake worth reporting rather than silently
// letting one of the two win.
func (f *Factory) refRepo(ref Ref) (Repo, error) {
	if ref.Repo == (Repo{}) {
		return f.BaseRepo()
	}
	repo, err := f.fillHost(ref.Repo)
	if err != nil {
		return Repo{}, err
	}
	if f.RepoOverride == "" {
		return repo, nil
	}
	override, err := f.RepoFromArg(f.RepoOverride)
	if err != nil {
		return Repo{}, err
	}
	if !sameRepo(repo, override) {
		return Repo{}, FlagErrorf(
			"the reference names %s/%s but -R names %s/%s",
			repo.Host, repo.FullName(), override.Host, override.FullName())
	}
	return repo, nil
}

func sameRepo(a, b Repo) bool {
	return strings.EqualFold(a.Host, b.Host) &&
		strings.EqualFold(a.Owner, b.Owner) &&
		strings.EqualFold(a.Name, b.Name)
}

// ListOpenPRs returns the repository's open pull requests, newest first,
// within the paging bounds above. Callers that need several views of the
// same set — a branch lookup and a poster filter, say — share one call
// rather than paging twice.
func (f *Factory) ListOpenPRs(repo Repo) ([]*forgejo.PullRequest, error) {
	client, err := f.ClientForRepo(repo)
	if err != nil {
		return nil, err
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
			return nil, fmt.Errorf("listing pull requests: %w", err)
		}
		all = append(all, prs...)
		if len(prs) < branchPRPageSize {
			break
		}
	}
	return all, nil
}

// FindBranchPR resolves the open pull request whose head is branch, or nil
// when the branch has none. Commands that must have one turn the nil into
// an error; status reports render the absence instead.
func (f *Factory) FindBranchPR(repo Repo, branch string) (*forgejo.PullRequest, error) {
	prs, err := f.ListOpenPRs(repo)
	if err != nil {
		return nil, err
	}
	return PickBranchPR(prs, repo, branch)
}

// branchPR resolves the open pull request whose head is branch. It backs
// both the explicit branch reference and the no-argument current-branch
// default, so the two cannot disagree about what a branch resolves to.
func (f *Factory) branchPR(repo Repo, branch string) (Repo, int64, error) {
	pr, err := f.FindBranchPR(repo, branch)
	if err != nil {
		return Repo{}, 0, err
	}
	if pr == nil {
		return Repo{}, 0, fmt.Errorf("no open pull request found for branch %q in %s", branch, repo.FullName())
	}
	return repo, pr.Index, nil
}

// PickBranchPR chooses the pull request whose head is branch, or nil when
// none matches. Pull requests opened from the base repository itself win
// over same-named branches on forks, which are somebody else's work.
func PickBranchPR(prs []*forgejo.PullRequest, repo Repo, branch string) (*forgejo.PullRequest, error) {
	var local, foreign []*forgejo.PullRequest
	for _, pr := range prs {
		if pr.Head == nil || pr.Head.Ref != branch {
			continue
		}
		if pr.Head.Repository != nil && !strings.EqualFold(pr.Head.Repository.FullName, repo.FullName()) {
			foreign = append(foreign, pr)
		} else {
			local = append(local, pr)
		}
	}

	matches := local
	if len(matches) == 0 {
		matches = foreign
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return matches[0], nil
	}
	return nil, fmt.Errorf("branch %q has %d open pull requests (%s); specify one by number",
		branch, len(matches), joinNumbers(matches))
}

func joinNumbers(prs []*forgejo.PullRequest) string {
	parts := make([]string, len(prs))
	for i, pr := range prs {
		parts[i] = "#" + strconv.FormatInt(pr.Index, 10)
	}
	return strings.Join(parts, ", ")
}
