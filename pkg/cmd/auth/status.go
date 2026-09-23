package auth

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type statusOptions struct {
	Factory    *cmdutil.Factory
	Hostname   string
	JSONOutput cmdutil.JSONFlags
}

func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	opts := &statusOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "View authentication status",
		Example: `  $ fj auth status
  $ fj auth status --hostname forgejo.example.com
  $ fj auth status --json hosts
  $ fj auth status --json hosts --jq '.hosts | add | .[].login'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "Check status for a specific host")
	cmdutil.AddJSONFlagsLong(cmd, &opts.JSONOutput, []string{"hosts"}, nil)

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
		Hostname    string
		User        string
		GitProtocol string
		Error       string
	}

	var statuses []hostStatus

	target := opts.Hostname
	if target == "" {
		target = os.Getenv("FJ_HOST")
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Hosts)) {
		host := cfg.Hosts[name]
		if opts.Hostname != "" && name != opts.Hostname {
			continue
		}

		status := hostStatus{
			Hostname:    name,
			User:        host.User,
			GitProtocol: cfg.GitProtocol(name),
		}

		// Verify the token still works
		client, err := cmdutil.NewForgejoClient(statusURL(name, target), host.Token)
		if err != nil {
			status.Error = err.Error()
		} else {
			user, _, err := client.GetMyUserInfo()
			if err != nil {
				status.Error = fmt.Sprintf("authentication failed: %s", err)
			} else {
				status.User = user.UserName
			}
		}

		statuses = append(statuses, status)
	}

	// fj keeps one account per host, so each host's one account is active.
	if opts.JSONOutput.Enabled() {
		hosts := map[string]any{}
		for _, s := range statuses {
			account := map[string]any{"active": true, "gitProtocol": s.GitProtocol, "host": s.Hostname, "login": s.User, "state": "success"}
			if s.Error != "" {
				account["state"], account["error"] = "error", s.Error
			}
			hosts[s.Hostname] = []map[string]any{account}
		}
		return opts.JSONOutput.Write(os.Stdout, map[string]any{"hosts": hosts})
	}

	for _, s := range statuses {
		fmt.Fprintf(os.Stdout, "%s\n", s.Hostname)
		fmt.Fprintf(os.Stdout, "  ✓ Logged in as %s\n", s.User)
		fmt.Fprintf(os.Stdout, "  ✓ Git protocol: %s\n", s.GitProtocol)
		if s.Error != "" {
			fmt.Fprintf(os.Stdout, "  ✗ Token invalid: %s\n", s.Error)
		}
		fmt.Fprintln(os.Stdout)
	}

	return nil
}

// statusURL is the URL that checks name's token. FJ_INSECURE applies only to
// target, the host named by --hostname or FJ_HOST: set for a dev instance, it
// must not send every other host's token over plain http.
func statusURL(name, target string) string {
	if name == target {
		return cmdutil.BaseURL(name)
	}
	return "https://" + name
}
