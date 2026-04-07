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

You can also set `FJ_TOKEN` to override the token for any command.

## Usage

`fj` auto-detects the repository from your git remotes. Use `-R OWNER/REPO` to override.

### Repositories

```sh
fj repo list                              # List your repos
fj repo list myorg                        # List repos for an org
fj repo view                              # View current repo
fj repo view owner/repo                   # View a specific repo
fj repo create my-project --private       # Create a new repo
fj repo clone owner/repo                  # Clone a repo
fj repo fork owner/repo                   # Fork a repo
fj repo delete owner/repo                 # Delete a repo
```

### Issues

```sh
fj issue list                             # List open issues
fj issue list --state closed              # List closed issues
fj issue list --label bug --assignee me   # Filter by label/assignee
fj issue view 42                          # View issue #42
fj issue create --title "Bug" --body "…"  # Create an issue
fj issue close 42                         # Close an issue
fj issue reopen 42                        # Reopen an issue
fj issue comment 42 --body "Fixed in …"   # Comment on an issue
fj issue edit 42 --title "New title"      # Edit an issue
```

### Pull Requests

```sh
fj pr list                                # List open PRs
fj pr list --state all                    # List all PRs
fj pr view 10                             # View PR #10
fj pr create --title "Fix" --body "…"     # Create a PR
fj pr merge 10                            # Merge a PR
fj pr close 10                            # Close a PR
fj pr diff 10                             # View PR diff
fj pr checkout 10                         # Check out a PR locally
fj pr comment 10 --body "LGTM"           # Comment on a PR
```

### JSON Output

Most commands support `--json` for machine-readable output:

```sh
fj repo list --json
fj issue view 42 --json
fj pr list --json | jq '.[].title'
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
| `FJ_TOKEN` | API token (overrides config file) |
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
