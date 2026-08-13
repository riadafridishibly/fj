package cmdutil

import (
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Ref is a parsed positional reference to an issue or pull request. It is
// pure syntax: nothing here consults configuration, git, or the network,
// so the repository it names may still lack a host.
type Ref struct {
	// Repo is the zero value when the reference carried no repository.
	Repo Repo
	// Number is 0 when the reference names a branch.
	Number int64
	// Branch is set only by the branch form, which pull request commands
	// resolve to an index and issue commands reject.
	Branch string
}

// RefKind is the subject a command resolves references against. Its value
// is that subject's web route, which is what the parse error shows as its
// example URL, so the advice matches the command the user ran.
type RefKind string

const (
	IssueRefKind RefKind = "issues"
	PRRefKind    RefKind = "pulls"
)

// refPaths are the web routes that address an issue or pull request. Both
// are accepted whatever the kind, since a URL identifies its own subject;
// gh's singular /pull/<n> works too so URLs pasted out of habit still do.
var refPaths = []string{"issues", "pulls", "pull"}

// ParseRef parses a positional issue or pull request reference in every
// form gh accepts — a bare number and a web URL — plus fj's owner/repo#42
// for addressing another repository without -R. Anything left over is
// taken to be a branch name, but a malformed URL or #-reference is an
// error rather than a branch, so a typo cannot become a branch lookup.
func ParseRef(arg string, kind RefKind) (Ref, error) {
	switch {
	case strings.HasPrefix(arg, "http://"), strings.HasPrefix(arg, "https://"):
		return parseRefURL(arg, kind)
	case strings.Contains(arg, "#"):
		return parseRefHash(arg, kind)
	case arg == "":
		return Ref{}, refError(arg, kind)
	}

	number, err := strconv.ParseInt(arg, 10, 64)
	if errors.Is(err, strconv.ErrSyntax) {
		// "42x" is not a number but is a legal branch name.
		return Ref{Branch: arg}, nil
	}
	if err != nil || number < 1 {
		return Ref{}, refError(arg, kind)
	}
	return Ref{Number: number}, nil
}

func parseRefURL(arg string, kind RefKind) (Ref, error) {
	u, err := url.Parse(arg)
	if err != nil || u.Host == "" {
		return Ref{}, refError(arg, kind)
	}

	// Segments past the number (/comments, a #issuecomment- fragment, a
	// query) address something within the issue, which is still that issue.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || !slices.Contains(refPaths, parts[2]) {
		return Ref{}, refError(arg, kind)
	}
	number, ok := refNumber(parts[3])
	if !ok {
		return Ref{}, refError(arg, kind)
	}
	return Ref{
		Repo:   Repo{Host: u.Host, Owner: parts[0], Name: parts[1]},
		Number: number,
	}, nil
}

func parseRefHash(arg string, kind RefKind) (Ref, error) {
	name, num, _ := strings.Cut(arg, "#")
	number, ok := refNumber(num)
	if !ok {
		return Ref{}, refError(arg, kind)
	}
	if name == "" {
		return Ref{Number: number}, nil
	}
	repo, err := RepoFromFullName(name)
	if err != nil {
		return Ref{}, refError(arg, kind)
	}
	return Ref{Repo: repo, Number: number}, nil
}

func refNumber(s string) (int64, bool) {
	number, err := strconv.ParseInt(s, 10, 64)
	return number, err == nil && number > 0
}

func refError(arg string, kind RefKind) error {
	return FlagErrorf(
		"invalid reference %q: expected a number, a URL such as "+
			"https://HOST/OWNER/REPO/%s/42, or OWNER/REPO#42", arg, kind)
}
