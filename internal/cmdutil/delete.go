package cmdutil

import "github.com/spf13/cobra"

// DryRunMessage is the standard notice printed when a delete command resolves
// its target but leaves it intact because --dry-run was given. Centralizing it
// keeps the wording identical across all delete commands.
const DryRunMessage = "(dry-run; no changes were made)"

// AddDeleteFlags registers the two flags every delete command shares:
//
//	--yes     perform the deletion
//	--dry-run resolve and print the target without deleting
//
// Exactly one is required when the command runs; see ResolveDeleteFlags.
func AddDeleteFlags(cmd *cobra.Command, yes, dryRun *bool) {
	cmd.Flags().BoolVar(yes, "yes", false, "Perform the deletion (required)")
	cmd.Flags().BoolVar(dryRun, "dry-run", false, "Resolve and show the target without deleting")
}

// ResolveDeleteFlags interprets the --yes / --dry-run flags shared by every
// delete command. Exactly one must be given:
//
//   - --yes     -> perform == true  (the caller executes the deletion)
//   - --dry-run -> perform == false (the caller resolves and prints, then stops)
//
// Passing both, or neither, is a flag error. There is no interactive prompt;
// --yes is the only way to actually delete.
func ResolveDeleteFlags(yes, dryRun bool) (perform bool, err error) {
	if yes && dryRun {
		return false, FlagErrorf("--yes and --dry-run are mutually exclusive")
	}
	if !yes && !dryRun {
		return false, FlagErrorf("refusing to delete without confirmation: pass --yes to delete or --dry-run to preview")
	}
	return yes, nil
}
