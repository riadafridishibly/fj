package cmdutil

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

// AddRepoOverrideFlags adds the -R [HOST/]OWNER/REPO flag to a command
func AddRepoOverrideFlags(cmd *cobra.Command, f *Factory) {
	cmd.PersistentFlags().StringVarP(&f.RepoOverride, "repo", "R", "", "Select a repository using the [HOST/]OWNER/REPO format")
}

// AddJSONFlag adds a --json flag for machine-readable output
func AddJSONFlag(cmd *cobra.Command, jsonOutput *bool) {
	cmd.Flags().BoolVar(jsonOutput, "json", false, "Output in JSON format")
}

// AddWebFlag adds a --web flag for opening in browser
func AddWebFlag(cmd *cobra.Command, web *bool) {
	cmd.Flags().BoolVarP(web, "web", "w", false, "Open in web browser")
}

// ReadBodyFromFile reads content from a file path for --body-file flags
func ReadBodyFromFile(path string) (string, error) {
	if path == "-" {
		data, err := readStdin()
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading body file: %w", err)
	}
	return string(data), nil
}

func readStdin() ([]byte, error) {
	return os.ReadFile("/dev/stdin")
}

// OpenInBrowser opens a URL in the default browser
func OpenInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}
