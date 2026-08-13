package cmdutil

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// ExactArgs wraps cobra.ExactArgs so a count mismatch prints usage and is
// reported as a usage error (exit code 2). Default cobra behavior with
// SilenceUsage=true only shows the bare "accepts N arg(s), received M"
// error, which is hard to act on.
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
			return &FlagError{err: err}
		}
		return nil
	}
}

// GroupDispatchArgs validates arguments for a group command that doubles as
// its numeric leaf verb (`fj issue comment 42 --body x`): at most one
// argument, which must look like a number — any other word is a mistyped
// subcommand, reported the way cobra reports unknown commands.
func GroupDispatchArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		}
	}
	return MaximumNArgs(1)(cmd, args)
}
