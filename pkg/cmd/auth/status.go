package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type statusOptions struct {
	Factory    *cmdutil.Factory
	Hostname   string
	JSONOutput bool
}

func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	opts := &statusOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "View authentication status",
		Example: `  $ fj auth status
  $ fj auth status --hostname forgejo.example.com
  $ fj auth status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "Check status for a specific host")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func statusRun(opts *statusOptions) error {
	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}

	if len(cfg.Hosts) == 0 {
		fmt.Fprintln(os.Stderr, "You are not logged in to any Forgejo hosts. Run 'fj auth login' to authenticate.")
		return cmdutil.ErrSilent
	}

	type hostStatus struct {
		Hostname    string `json:"hostname"`
		User        string `json:"user"`
		GitProtocol string `json:"git_protocol"`
		Active      bool   `json:"active"`
		Error       string `json:"error,omitempty"`
	}

	var statuses []hostStatus

	for name, host := range cfg.Hosts {
		if opts.Hostname != "" && name != opts.Hostname {
			continue
		}

		status := hostStatus{
			Hostname:    name,
			User:        host.User,
			GitProtocol: host.GitProtocol,
			Active:      true,
		}

		// Verify the token still works
		baseURL := fmt.Sprintf("https://%s", name)
		client, err := cmdutil.NewForgejoClient(baseURL, host.Token)
		if err != nil {
			status.Active = false
			status.Error = err.Error()
		} else {
			user, _, err := client.GetMyUserInfo()
			if err != nil {
				status.Active = false
				status.Error = fmt.Sprintf("authentication failed: %s", err)
			} else {
				status.User = user.UserName
			}
		}

		statuses = append(statuses, status)
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(statuses)
	}

	for _, s := range statuses {
		fmt.Fprintf(os.Stdout, "%s\n", s.Hostname)
		fmt.Fprintf(os.Stdout, "  ✓ Logged in as %s\n", s.User)
		fmt.Fprintf(os.Stdout, "  ✓ Git protocol: %s\n", s.GitProtocol)
		if !s.Active {
			fmt.Fprintf(os.Stdout, "  ✗ Token invalid: %s\n", s.Error)
		}
		fmt.Fprintln(os.Stdout)
	}

	return nil
}
