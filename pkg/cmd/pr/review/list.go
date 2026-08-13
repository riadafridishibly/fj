package review

import (
	"fmt"
	"os"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type listOptions struct {
	Factory    *cmdutil.Factory
	Args       []string
	Limit      int
	JSONOutput bool
}

func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &listOptions{Factory: f}

	cmd := &cobra.Command{
		Use:     "list [<number>]",
		Short:   "List reviews on a pull request",
		Aliases: []string{"ls"},
		Example: `  $ fj pr review list 42
  $ fj pr review list 42 --limit 100
  $ fj pr review list 42 --json`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of reviews to list")
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func listRun(opts *listOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, repo, err := opts.Factory.PRNumber(repo, opts.Args)
	if err != nil {
		return err
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	pageSize := min(opts.Limit, 50)
	listOpt := forgejo.ListPullReviewsOptions{
		ListOptions: forgejo.ListOptions{Page: 1, PageSize: pageSize},
	}

	var all []*forgejo.PullReview
	page := 1
	for len(all) < opts.Limit {
		listOpt.Page = page
		reviews, _, err := client.ListPullReviews(repo.Owner, repo.Name, index, listOpt)
		if err != nil {
			return fmt.Errorf("listing reviews: %w", err)
		}
		if len(reviews) == 0 {
			break
		}
		all = append(all, reviews...)
		if len(reviews) < pageSize {
			break
		}
		page++
	}
	if len(all) > opts.Limit {
		all = all[:opts.Limit]
	}

	if opts.JSONOutput {
		return output.PrintJSON(os.Stdout, all)
	}

	if len(all) == 0 {
		fmt.Fprintf(os.Stderr, "No reviews on pull request #%d in %s\n", index, repo.FullName())
		return nil
	}

	t := output.NewTable("ID", "STATE", "REVIEWER", "COMMENTS", "SUBMITTED")
	for _, r := range all {
		reviewer := ""
		if r.Reviewer != nil {
			reviewer = r.Reviewer.UserName
		} else if r.ReviewerTeam != nil {
			reviewer = "@" + r.ReviewerTeam.Name
		}

		state := string(r.State)
		color := stateColor(r.State)

		t.AddRow(
			strconv.FormatInt(r.ID, 10),
			output.Colorize(color, state),
			reviewer,
			strconv.Itoa(r.CodeCommentsCount),
			output.Colorize(output.Gray, output.RelativeTimeStr(r.Submitted)),
		)
	}
	t.Render(os.Stdout)
	return nil
}

func stateColor(s forgejo.ReviewStateType) string {
	switch s {
	case forgejo.ReviewStateApproved:
		return output.Green
	case forgejo.ReviewStateRequestChanges:
		return output.Red
	case forgejo.ReviewStatePending:
		return output.Yellow
	default:
		return output.Cyan
	}
}
