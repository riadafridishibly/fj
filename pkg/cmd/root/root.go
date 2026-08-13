package root

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	authCmd "github.com/riadafridishibly/fj/pkg/cmd/auth"
	issueCmd "github.com/riadafridishibly/fj/pkg/cmd/issue"
	labelCmd "github.com/riadafridishibly/fj/pkg/cmd/label"
	milestoneCmd "github.com/riadafridishibly/fj/pkg/cmd/milestone"
	prCmd "github.com/riadafridishibly/fj/pkg/cmd/pr"
	releaseCmd "github.com/riadafridishibly/fj/pkg/cmd/release"
	repoCmd "github.com/riadafridishibly/fj/pkg/cmd/repo"
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
	cmd.AddCommand(newCmdRemovedStatus())

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

// newCmdRemovedStatus keeps `fj status` from degrading into "unknown
// command" while the name changes hands: the repository summary it used to
// print now lives at `fj repo status`, and the name itself is reserved for
// gh's cross-repository meaning. Hidden, so help does not advertise a
// command that only ever fails.
//
// The next slice replaces this with the real cross-repository status.
func newCmdRemovedStatus() *cobra.Command {
	return &cobra.Command{
		Use:    "status",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.FlagErrorf(
				"`fj status` no longer prints the repository summary — that moved to `fj repo status`.\n" +
					"The name is being reworked to match `gh status`: issues, pull requests and mentions " +
					"relevant to you across repositories.")
		},
	}
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
