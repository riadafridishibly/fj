# fj

A command-line tool for working with [Forgejo](https://forgejo.org) — like `gh` for GitHub, but for Forgejo instances.

## Installation

```sh
go install github.com/riadafridishibly/fj/cmd/fj@latest
```

Or install a release binary from GitHub Releases:

```sh
curl -fsSL https://raw.githubusercontent.com/riadafridishibly/fj/dev/scripts/install.sh | sh
```

Install to a specific directory (default: `~/.local/bin`):

```sh
curl -fsSL https://raw.githubusercontent.com/riadafridishibly/fj/dev/scripts/install.sh | FJ_INSTALL_DIR=/usr/local/bin sh
```

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/riadafridishibly/fj/dev/scripts/install.sh | FJ_VERSION=v0.1.0 sh
```

Both options can be combined:

```sh
curl -fsSL https://raw.githubusercontent.com/riadafridishibly/fj/dev/scripts/install.sh | FJ_INSTALL_DIR=$HOME/bin FJ_VERSION=v0.1.0 sh
```

> **Note:** The install script supports macOS and Linux. For Windows and FreeBSD, download the appropriate binary from [GitHub Releases](https://github.com/riadafridishibly/fj/releases).

Or build from source:

```sh
git clone https://github.com/riadafridishibly/fj.git
cd fj
go build -o fj ./cmd/fj
```

## Authentication

```sh
# Interactive login
fj auth login --hostname forgejo.example.com

# Login with a token directly
fj auth login --hostname forgejo.example.com --token YOUR_TOKEN

# Pipe a token from stdin
echo YOUR_TOKEN | fj auth login --hostname forgejo.example.com --with-token

# Check auth status
fj auth status
```

Configuration is stored in `~/.config/fj/config.yaml` (or `$FJ_CONFIG_DIR`).

You can also set `FJ_TOKEN` to use a token without logging in. It applies to one host only: the one named by `FJ_HOST`, or the only configured host. With several hosts configured, `FJ_TOKEN` also needs `FJ_HOST`. In CI, where there is usually no config file, set both:

```sh
FJ_HOST=forgejo.example.com FJ_TOKEN=... fj issue list -R owner/repo
```

## Usage

`fj` auto-detects the repository from your git remotes. Use `-R OWNER/REPO` to override.

When a command needs a host but its arguments name none, fj uses, in order: `--hostname` on commands that have it, `FJ_HOST`, the host of the current checkout's git remote, and the only configured host. With several hosts configured and none of these set, the command fails rather than guess.

### Repositories

```sh
fj repo list                              # List your repos
fj repo list myorg                        # List repos for an org
fj repo view                              # View current repo
fj repo view owner/repo                   # View a specific repo
fj repo create my-project --private       # Create a new repo
fj repo clone owner/repo                  # Clone a repo
fj repo fork owner/repo                   # Fork a repo
fj repo delete owner/repo --yes           # Delete (needs --yes; --dry-run previews)
```

### Issues

```sh
fj issue list                             # List open issues
fj issue list --state closed              # List closed issues
fj issue list --label bug --assignee me   # Filter by label/assignee
fj issue view 42                          # View issue #42, with its timeline
fj issue view 42 --show-timeline=false    # View issue #42 without events
fj issue view 42 \
  --timeline-exclude commits              # Hide commit references (often the bulk)
fj issue view 42 \
  --timeline-include refs                 # Show only what referenced this issue
fj issue view 42 --json title,labels      # JSON with gh's field names
fj issue create --title "Bug" --body "…"  # Create an issue
fj issue close 42                         # Close an issue
fj issue reopen 42                        # Reopen an issue
fj issue comment 42 --body "Fixed in …"   # Comment on an issue
fj issue edit 42 --title "New title"      # Edit an issue
fj issue attach 42 screenshot.png         # Attach a file, print its URL
```

### Milestones

```sh
fj milestone list                         # List open milestones
fj milestone list --state all             # List all milestones
fj milestone view v1.0                    # View a milestone
fj milestone create --title v1.0 \
  --due-date 2026-12-31                   # Create a milestone
fj milestone edit v1.0 --title v1.1       # Edit a milestone
fj milestone close v1.0                   # Close a milestone
fj milestone reopen v1.0                  # Reopen a milestone
fj milestone delete v1.0 --yes            # Delete (needs --yes; --dry-run previews)
```

### Pull Requests

```sh
fj pr list                                # List open PRs
fj pr list --state all                    # List all PRs
fj pr view 10                             # View PR #10, with its timeline
fj pr view 10 --show-timeline=false       # View PR #10 without events
fj pr view 10 --timeline-exclude commits  # Hide commit references
fj pr create --title "Fix" --body "…"     # Create a PR
fj pr merge 10                            # Merge a PR
fj pr close 10                            # Close a PR
fj pr diff 10                             # View PR diff
fj pr checkout 10                         # Check out a PR locally
fj pr comment 10 --body "LGTM"           # Comment on a PR
```

### Reviews

```sh
fj pr review create 10 --approve --body "LGTM"                # Review a PR
fj pr review create 10 --comment \
  --comment-path main.go --comment-line 42 \
  --comment-body "rename this"                                # Inline comment
fj pr review list 10                                          # List reviews
fj pr review comment list 10                                  # List inline comments
fj pr review comment view 10 4081                             # View one comment
fj pr review comment reply 10 4081 --body "Fixed"             # Reply in-thread
fj pr review comment delete 10 4081 --yes                     # Delete (needs --yes; --dry-run previews)
```

### JSON Output

`fj issue list`, `view`, `create` and `edit` take gh's formatting flags: `--json <fields>`, `-q/--jq` and (list and view) `-t/--template`. Field names and shapes follow gh's, so a gh script reads the output unchanged. Each command's `--help` lists its fields, and `fj help formatting` covers the flags:

```sh
fj issue list --json number,title,labels
fj issue view 42 --json author,state --jq '.author.login'
fj issue list --json number,title --template '{{range .}}#{{.number}} {{.title}}{{"\n"}}{{end}}'
```

Other commands still take a bare `--json` and print Forgejo's own JSON:

```sh
fj repo list --json
fj pr list --json | jq '.[].title'
```

### API

`fj api` calls any endpoint of the Forgejo API, including the ones no other command covers. It works like `gh api`. A server's full API reference is at `https://<host>/swagger.v1.json`.

```sh
fj api repos/{owner}/{repo}/releases                          # {owner}/{repo} from the current repo
fj api repos/{owner}/{repo}/issues/42/comments -f body='Hi'   # Fields switch the method to POST
fj api --paginate 'repos/{owner}/{repo}/issues?limit=50' --jq '.[].title'
fj api -X PATCH repos/{owner}/{repo}/labels/7 -F exclusive=true
```

### Open in Browser

View commands support `--web` to open in your browser:

```sh
fj issue view 42 --web
fj pr view 10 --web
fj repo view --web
```

## Environment Variables

| Variable | Description |
|---|---|
| `FJ_TOKEN` | API token, used instead of the stored one for a single host: `FJ_HOST` if set, otherwise the only configured host |
| `FJ_HOST` | Host to use when a command names none. It takes priority over the checkout's host and the configured hosts |
| `FJ_CONFIG_DIR` | Config directory (default: `~/.config/fj`) |
| `FJ_INSECURE` | Use HTTP instead of HTTPS (for local/dev instances) |

## Integration Tests

The project includes integration tests that spin up a real Forgejo instance using [testcontainers-go](https://github.com/testcontainers/testcontainers-go). Requires Docker.

```sh
go test -tags integration -v -timeout 120s ./integration/
```

## Releases

Releases should be tag-driven. Push a semantic version tag such as `v0.1.0` and GitHub Actions will build archives for macOS, Linux, Windows, and FreeBSD (amd64 + arm64), publish a GitHub Release, and attach SHA-256 checksums.

```sh
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```
