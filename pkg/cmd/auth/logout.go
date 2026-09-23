package auth

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

type logoutOptions struct {
	Factory  *cmdutil.Factory
	Hostname string
}

func NewCmdLogout(f *cmdutil.Factory) *cobra.Command {
	opts := &logoutOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of a Forgejo host",
		Example: `  $ fj auth logout --hostname forgejo.example.com
  $ fj auth logout`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return logoutRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "The hostname to log out of")

	return cmd
}

func logoutRun(opts *logoutOptions) error {
	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}

	// Removing credentials is not inferred from FJ_HOST or the checkout, as
	// other commands infer their host: without --hostname, only a single
	// configured host is unambiguous enough to log out of.
	hostname := opts.Hostname
	if hostname == "" {
		if hostname, err = cfg.OnlyHost("pass --hostname to choose one"); err != nil {
			return err
		}
	}

	if _, ok := cfg.Hosts[hostname]; !ok {
		return fmt.Errorf("not logged in to %s", hostname)
	}

	delete(cfg.Hosts, hostname)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Logged out of %s\n", hostname)
	fmt.Fprintf(os.Stderr, "  Config saved to %s\n", config.ConfigPath())
	return nil
}
