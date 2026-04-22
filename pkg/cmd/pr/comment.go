package pr

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/comment"
)

func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	return comment.NewCmdComment(f, comment.Kind{
		Noun: "pull request",
		CLI:  "fj pr comment",
		Arg:  "pr",
	})
}
