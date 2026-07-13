package main

import (
	"fmt"
	"os"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/root"
)

func main() {
	f := cmdutil.NewFactory()
	rootCmd := root.NewCmdRoot(f)

	if err := rootCmd.Execute(); err != nil {
		if cmdutil.IsFlagError(err) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(2)
		}
		if err != cmdutil.ErrSilent {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		os.Exit(1)
	}
}
