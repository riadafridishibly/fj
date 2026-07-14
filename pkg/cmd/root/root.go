package root

import (
	"fmt"
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
	statusCmd "github.com/riadafridishibly/fj/pkg/cmd/status"
)

var (
	Version = "dev"
	Commit  = ""
)

func NewCmdRoot(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fj <command> <subcommand> [flags]",
		Short: "Forgejo CLI",
		Long:  "Work seamlessly with Forgejo from the command line.",
		Example: `  $ fj issue list
  $ fj pr create --title "Fix bug" --body "Description"
  $ fj repo view
  $ fj auth login --hostname forgejo.example.com`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().Bool("help", false, "Show help for command")

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
