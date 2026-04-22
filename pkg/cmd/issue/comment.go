package issue

import (
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/pkg/cmd/comment"
)

func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	return comment.NewCmdComment(f, comment.Kind{
		Noun: "issue",
		CLI:  "fj issue comment",
		Arg:  "issue",
	})
}
