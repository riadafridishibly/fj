package git

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
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
	return exec.Command("git", "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}").Run() == nil
}

// SupportsMergeTree reports whether git has merge-tree --write-tree (2.38+),
// along with the detected version. Unparseable versions count as supported.
func SupportsMergeTree() (bool, string) {
	out, err := Run("version")
	if err != nil {
		return true, ""
	}
	v := strings.TrimPrefix(out, "git version ")
	if f := strings.Fields(v); len(f) > 0 {
		v = f[0]
	}
	return versionAtLeast(v, 2, 38), v
}

func versionAtLeast(v string, major, minor int) bool {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return true
	}
	gotMajor, majErr := strconv.Atoi(parts[0])
	gotMinor, minErr := strconv.Atoi(parts[1])
	if majErr != nil || minErr != nil {
		return true
	}
	return gotMajor > major || (gotMajor == major && gotMinor >= minor)
}

// MergeTreeConflicts merges head into base in memory (git merge-tree, >= 2.38)
// without touching the working tree, index, or HEAD. A non-nil error means the
// result is unknown, not clean.
func MergeTreeConflicts(base, head string) (files []string, clean bool, err error) {
	cmd := exec.Command("git", "merge-tree", "--write-tree", "-z", "--name-only", "--no-messages", "--end-of-options", base, head)
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

	// Conflicts emit <tree-OID>NUL<path>NUL...; a rejected input (bad rev,
	// unrelated histories) emits nothing, so a missing OID means failure.
	records := strings.Split(stdout.String(), "\x00")
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

func Fetch(remote, refspec string) error {
	_, err := Run("fetch", remote, refspec)
	return err
}
