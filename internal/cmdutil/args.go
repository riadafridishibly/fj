package cmdutil

import "github.com/spf13/cobra"

// ExactArgs wraps cobra.ExactArgs so usage is printed when the arg count
// doesn't match. Default cobra behavior with SilenceUsage=true only shows
// the bare "accepts N arg(s), received M" error, which is hard to act on.
func ExactArgs(n int) cobra.PositionalArgs { return wrapArgs(cobra.ExactArgs(n)) }

// MinimumNArgs wraps cobra.MinimumNArgs with usage-on-failure.
func MinimumNArgs(n int) cobra.PositionalArgs { return wrapArgs(cobra.MinimumNArgs(n)) }

// MaximumNArgs wraps cobra.MaximumNArgs with usage-on-failure.
func MaximumNArgs(n int) cobra.PositionalArgs { return wrapArgs(cobra.MaximumNArgs(n)) }

// RangeArgs wraps cobra.RangeArgs with usage-on-failure.
func RangeArgs(min, max int) cobra.PositionalArgs { return wrapArgs(cobra.RangeArgs(min, max)) }

func wrapArgs(validator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validator(cmd, args); err != nil {
			_ = cmd.Usage()
			return err
		}
		return nil
	}
}
