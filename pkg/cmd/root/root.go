package root

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	authCmd "github.com/riadafridishibly/fj/pkg/cmd/auth"
	issueCmd "github.com/riadafridishibly/fj/pkg/cmd/issue"
	prCmd "github.com/riadafridishibly/fj/pkg/cmd/pr"
	repoCmd "github.com/riadafridishibly/fj/pkg/cmd/repo"
)

var Version = "dev"

func NewCmdRoot(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fj <command> <subcommand> [flags]",
		Short: "Forgejo CLI",
		Long:  "Work seamlessly with Forgejo from the command line.",
		Example: `  $ fj issue list
  $ fj pr create --title "Fix bug" --body "Description"
  $ fj repo view
  $ fj auth login --hostname code.evatix.com`,
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
	cmd.AddCommand(prCmd.NewCmdPR(f))

	// Version command
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("fj version %s\n", Version)
		},
	})

	return cmd
}
