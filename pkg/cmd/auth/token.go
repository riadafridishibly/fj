package auth

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

type tokenOptions struct {
	Factory  *cmdutil.Factory
	Hostname string
}

func NewCmdToken(f *cmdutil.Factory) *cobra.Command {
	opts := &tokenOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Print the authentication token for a host",
		Long: `Print the authentication token for a Forgejo host.

This token can be used to authenticate API requests.
The output is suitable for piping to other commands.`,
		Example: `  $ fj auth token
  $ fj auth token --hostname code.evatix.com
  $ fj auth token | pbcopy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tokenRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "Get token for a specific host")

	return cmd
}

func tokenRun(opts *tokenOptions) error {
	cfg, err := opts.Factory.Config()
	if err != nil {
		return err
	}

	var host string
	if opts.Hostname != "" {
		host = opts.Hostname
	} else {
		_, h, err := cfg.DefaultHost()
		if err != nil {
			return err
		}
		host = h
	}

	token, err := cfg.TokenForHost(host)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, token)
	return nil
}
