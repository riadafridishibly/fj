package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

type loginOptions struct {
	Factory     *cmdutil.Factory
	Hostname    string
	Token       string
	User        string
	GitProtocol string
	WithToken   bool
}

func NewCmdLogin(f *cmdutil.Factory) *cobra.Command {
	opts := &loginOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a Forgejo host",
		Long: `Authenticate with a Forgejo host.

The token can be provided via --token flag, piped via stdin with --with-token,
or set via the FJ_TOKEN environment variable.`,
		Example: `  # Interactive login
  $ fj auth login --hostname forgejo.example.com

  # Login with token directly
  $ fj auth login --hostname forgejo.example.com --token <token>

  # Login with token from stdin
  $ echo <token> | fj auth login --hostname forgejo.example.com --with-token

  # Login with all options
  $ fj auth login --hostname forgejo.example.com --token <token> --git-protocol ssh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return loginRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "The hostname of the Forgejo instance to authenticate with")
	cmd.Flags().StringVar(&opts.Token, "token", "", "Authentication token")
	cmd.Flags().StringVar(&opts.GitProtocol, "git-protocol", "https", "The protocol to use for git operations (https or ssh)")
	cmd.Flags().BoolVar(&opts.WithToken, "with-token", false, "Read token from standard input")

	return cmd
}

func loginRun(opts *loginOptions) error {
	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}

	hostname := opts.Hostname
	if hostname == "" {
		hostname, err = promptString("Hostname (e.g. forgejo.example.com): ")
		if err != nil {
			return err
		}
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return cmdutil.FlagErrorf("hostname is required")
	}

	token := opts.Token
	if token == "" && opts.WithToken {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			token = strings.TrimSpace(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading token from stdin: %w", err)
		}
	}
	if token == "" {
		token, err = promptString("Token: ")
		if err != nil {
			return err
		}
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return cmdutil.FlagErrorf("token is required")
	}

	// Validate the token by fetching user info
	baseURL := fmt.Sprintf("https://%s", hostname)
	client, err := cmdutil.NewForgejoClient(baseURL, token)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	user, _, err := client.GetMyUserInfo()
	if err != nil {
		return fmt.Errorf("authentication failed: %w. Check that your token is valid", err)
	}

	cfg.Hosts[hostname] = &config.HostConfig{
		Hostname:    hostname,
		Token:       token,
		User:        user.UserName,
		GitProtocol: opts.GitProtocol,
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Logged in to %s as %s\n", hostname, user.UserName)
	fmt.Fprintf(os.Stderr, "  Config saved to %s\n", config.ConfigPath())
	return nil
}

func promptString(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no input provided")
}
