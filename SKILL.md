---
name: fj
description: Work with Forgejo repositories, issues, pull requests, reviews, releases, labels and milestones through the fj CLI. Use when the git remote is a Forgejo host or the user mentions Forgejo or fj.
---

# fj

`fj` is the GitHub CLI (`gh`) for Forgejo. Commands, flags and `--json` field names follow `gh`, so use what you know about `gh` and check `fj <command> --help` when unsure.

## Rules

- Never run `fj auth login`. It is interactive. If a command fails on auth, ask the user to log in, or to set `FJ_HOST` and `FJ_TOKEN`.
- `fj` finds the repository from the git remotes. Pass `-R OWNER/REPO` for another repository and `-C <path>` to run from another directory.
- Read with `--json <fields>` and filter with `--jq`. Bare `--json` prints the valid fields and exits 1.
- Write bodies with `--body-file -` and a heredoc, not `--body`, so quotes and backticks survive the shell.
- Delete commands need `--yes`. Run them with `--dry-run` first and confirm with the user.
- Views include the issue or PR timeline. Add `--timeline-exclude commits` or `--show-timeline=false` when it is too long.
- Forgejo marks a draft PR with a `WIP:` title prefix. Create one with `fj pr create --draft`, mark it ready with `fj pr ready N`, and make it a draft again with `fj pr ready N --undo`.
- For anything no command covers, use `fj api`. It works like `gh api` and fills `{owner}` and `{repo}` from the current repository.

## Examples

```sh
fj issue list --json number,title,labels
fj issue view 42 --json title,body,comments --jq '.comments[].body'
fj pr view 10 --json files --jq '.files[].path'
fj pr diff 10
fj pr checkout 10

fj issue create --title "Crash on empty input" --body-file - <<'EOF'
Steps to reproduce: run `app < /dev/null`.
EOF
fj issue comment create 42 --body-file - <<'EOF'
Fixed in #43.
EOF
fj pr create --title "Fix crash on empty input" --base main --body-file - <<'EOF'
Closes #42.
EOF

fj pr review create 10 --approve --body "LGTM"
fj pr review create 10 --comment --comment-path main.go --comment-line 42 --comment-body "Rename this"
fj pr review comment reply 10 4081 --body "Done"
fj issue attach 42 screenshot.png

fj api 'repos/{owner}/{repo}/releases' --jq '.[].tag_name'
fj api -X PATCH 'repos/{owner}/{repo}/issues/42' -f state=closed
```
