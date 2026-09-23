package root

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	apiCmd "github.com/riadafridishibly/fj/pkg/cmd/api"
	authCmd "github.com/riadafridishibly/fj/pkg/cmd/auth"
	issueCmd "github.com/riadafridishibly/fj/pkg/cmd/issue"
	labelCmd "github.com/riadafridishibly/fj/pkg/cmd/label"
	milestoneCmd "github.com/riadafridishibly/fj/pkg/cmd/milestone"
	prCmd "github.com/riadafridishibly/fj/pkg/cmd/pr"
	releaseCmd "github.com/riadafridishibly/fj/pkg/cmd/release"
	repoCmd "github.com/riadafridishibly/fj/pkg/cmd/repo"
	statusCmd "github.com/riadafridishibly/fj/pkg/cmd/status"
)

var (
	Version = "dev"
	Commit  = ""
)

func NewCmdRoot(f *cmdutil.Factory) *cobra.Command {
	var directory string

	cmd := &cobra.Command{
		Use:   "fj <command> <subcommand> [flags]",
		Short: "Forgejo CLI",
		Long:  "Work seamlessly with Forgejo from the command line.",
		Example: `  $ fj issue list
  $ fj pr create --title "Fix bug" --body "Description"
  $ fj repo view
  $ fj -C ../other-repo issue list
  $ fj auth login --hostname forgejo.example.com`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if directory != "" {
				if err := os.Chdir(directory); err != nil {
					return fmt.Errorf("cannot change directory: %w", err)
				}
			}
			return nil
		},
	}

	cmd.PersistentFlags().Bool("help", false, "Show help for command")
	cmd.PersistentFlags().StringVarP(&directory, "directory", "C", "", "Run as if fj was started in `<path>` instead of the current working directory")

	// Add -R flag at root level
	cmdutil.AddRepoOverrideFlags(cmd, f)

	// Core commands
	cmd.AddCommand(authCmd.NewCmdAuth(f))
	cmd.AddCommand(repoCmd.NewCmdRepo(f))
	cmd.AddCommand(issueCmd.NewCmdIssue(f))
	cmd.AddCommand(labelCmd.NewCmdLabel(f))
	cmd.AddCommand(milestoneCmd.NewCmdMilestone(f))
	cmd.AddCommand(prCmd.NewCmdPR(f))
	cmd.AddCommand(releaseCmd.NewCmdRelease(f))
	cmd.AddCommand(statusCmd.NewCmdStatus(f))
	cmd.AddCommand(apiCmd.NewCmdAPI(f))

	cmd.AddCommand(&cobra.Command{
		Use:   "formatting",
		Short: "Formatting options for JSON data exported from fj",
		Long: `Commands with a JSON FIELDS section in their --help print JSON instead of
text when given --json. Field names and shapes follow gh (GitHub CLI), so a
script written for gh reads fj's output the same way. Fields GitHub has but
Forgejo lacks are left out; fields only fj has are listed apart as fj-only.
One difference: ids are Forgejo's numeric IDs, not GitHub's string node IDs.

--json takes a comma-separated list of fields. Without one, fj lists the
valid fields and exits 1. On a terminal the JSON is indented; otherwise it
is printed on one line.

-q/--jq filters the JSON with a jq expression. Strings print raw and other
values as compact JSON, one result per line. jq need not be installed.

-t/--template formats the JSON with a Go template (text/template). Both --jq
and --template need --json; given both, --jq wins. The template can use
these functions besides the standard ones:
- autocolor <style> <input>: like color, but only colors on a terminal
- color <style> <input>: colorize input, style being a color name such as green
- join <sep> <list>: join the values in list with sep
- pluck <field> <list>: collect field from every item in list
- tablerow <fields>...: align fields vertically as a table
- tablerender: print the rows added by tablerow so far
- timeago <time>: render a timestamp relative to now
- timefmt <format> <time>: format a timestamp with Go's Time.Format
- truncate <length> <input>: shorten input to fit length
- hyperlink <url> <text>: render a terminal hyperlink
- contains <arg> <string>, hasPrefix <prefix> <string>,
  hasSuffix <suffix> <string>, regexMatch <regex> <string>

Examples:
  $ fj issue list --json number,title,labels
  $ fj issue list --json author --jq '.[].author.login'
  $ fj issue list --json number,title,updatedAt --template \
      '{{range .}}{{tablerow .number .title (timeago .updatedAt)}}{{end}}'`,
	})

	// Version command
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			v, c := versionInfo()
			if c != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "fj version %s (%s)\n", v, c)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "fj version %s\n", v)
			}
		},
	})

	return cmd
}

func versionInfo() (version, commit string) {
	version = Version
	commit = Commit

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit
	}

	// If version wasn't set via ldflags, use the module version
	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	// If commit wasn't set via ldflags, try vcs.revision from build info
	if commit == "" {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				commit = s.Value
				if len(commit) > 12 {
					commit = commit[:12]
				}
				break
			}
		}
	}

	return version, commit
}
