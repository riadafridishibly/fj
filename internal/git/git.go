package git

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

type Remote struct {
	Name     string
	FetchURL string
	PushURL  string
}

type RepoInfo struct {
	Host  string
	Owner string
	Name  string
}

func Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func CurrentBranch() (string, error) {
	return Run("symbolic-ref", "--short", "HEAD")
}

// RevExists reports whether rev resolves to a commit in the local object database.
func RevExists(rev string) bool {
	if rev == "" {
		return false
	}
	return exec.Command("git", "rev-parse", "--verify", "--quiet", rev+"^{commit}").Run() == nil
}

// MergeTreeConflicts merges head into base in memory with `git merge-tree`
// (requires git >= 2.38) and reports the conflicting files without touching the
// working tree, index, or HEAD. clean is true for a conflict-free merge. A
// non-nil error (git too old, rev missing locally, unrelated histories, not a
// repo) means the result is unknown and must not be read as clean.
//
// The -z form keeps paths with spaces or newlines intact: git emits
// <tree-OID>NUL<path>NUL<path>..., and --no-messages drops the conflict report.
func MergeTreeConflicts(base, head string) (files []string, clean bool, err error) {
	cmd := exec.Command("git", "merge-tree", "--write-tree", "-z", "--name-only", "--no-messages", base, head)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr == nil {
		return nil, true, nil
	}
	if _, ok := runErr.(*exec.ExitError); !ok {
		return nil, false, fmt.Errorf("git merge-tree: %w", runErr)
	}

	records := strings.Split(stdout.String(), "\x00")
	// A conflicted merge exits 1 but still emits the tree OID first. So does a
	// rejected input (bad rev, unrelated histories) — but with empty stdout, so
	// a missing OID means failure, not a phantom conflict.
	if records[0] == "" {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		return nil, false, fmt.Errorf("git merge-tree %s %s: %s", base, head, msg)
	}

	for _, path := range records[1:] {
		if path != "" {
			files = append(files, path)
		}
	}
	return files, false, nil
}

func Remotes() ([]Remote, error) {
	out, err := Run("remote", "-v")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	remoteMap := make(map[string]*Remote)
	for line := range strings.SplitSeq(out, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		name, u, kind := parts[0], parts[1], parts[2]
		r, ok := remoteMap[name]
		if !ok {
			r = &Remote{Name: name}
			remoteMap[name] = r
		}
		switch kind {
		case "(fetch)":
			r.FetchURL = u
		case "(push)":
			r.PushURL = u
		}
	}

	var remotes []Remote
	for _, r := range remoteMap {
		remotes = append(remotes, *r)
	}
	return remotes, nil
}

func ParseRemoteURL(rawURL string) (*RepoInfo, error) {
	// Handle SSH URLs: git@host:owner/repo.git
	if after, ok := strings.CutPrefix(rawURL, "git@"); ok {
		rawURL = after
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("cannot parse SSH URL: %s", rawURL)
		}
		host := parts[0]
		path := strings.TrimSuffix(parts[1], ".git")
		ownerRepo := strings.SplitN(path, "/", 2)
		if len(ownerRepo) != 2 {
			return nil, fmt.Errorf("cannot parse repo path: %s", path)
		}
		return &RepoInfo{Host: host, Owner: ownerRepo[0], Name: ownerRepo[1]}, nil
	}

	// Handle HTTPS URLs
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URL: %s", rawURL)
	}
	path := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	ownerRepo := strings.SplitN(path, "/", 2)
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("cannot parse repo path: %s", path)
	}
	return &RepoInfo{Host: u.Hostname(), Owner: ownerRepo[0], Name: ownerRepo[1]}, nil
}

func Clone(cloneURL, directory string) error {
	args := []string{"clone", cloneURL}
	if directory != "" {
		args = append(args, directory)
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func Checkout(branch string) error {
	_, err := Run("checkout", branch)
	return err
}

func CreateBranch(branch, startPoint string) error {
	_, err := Run("checkout", "-b", branch, startPoint)
	return err
}

func Fetch(remote, refspec string) error {
	_, err := Run("fetch", remote, refspec)
	return err
}

func AddRemote(name, url string) error {
	_, err := Run("remote", "add", name, url)
	return err
}

func TopLevelDir() (string, error) {
	dir, err := Run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(dir), nil
}
