package issue

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/spf13/cobra"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/output"
)

type viewOptions struct {
	Factory      *cmdutil.Factory
	Number       string
	Comments     bool
	ShowTimeline bool
	Web          bool
	JSONOutput   bool
	Download     bool
	DownloadDir  string
}

func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	opts := &viewOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "view <number>",
		Short: "View an issue",
		Example: `  $ fj issue view 42
  $ fj issue view 42 --comments
  $ fj issue view 42 --show-timeline=false
  $ fj issue view 42 --web
  $ fj issue view 42 --json
  $ fj issue view 42 --download
  $ fj issue view 42 --download --download-dir ./tmp`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Number = args[0]
			return viewRun(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.Comments, "comments", "c", false, "View issue comments")
	cmd.Flags().BoolVar(&opts.ShowTimeline, "show-timeline", true, "Show issue events, such as commit references and label changes")
	cmd.Flags().BoolVar(&opts.Download, "download", false, "Download issue attachments")
	cmd.Flags().StringVarP(&opts.DownloadDir, "download-dir", "D", "", "Directory to download attachments into (default: ./issue-<number>)")
	cmdutil.AddWebFlag(cmd, &opts.Web)
	cmdutil.AddJSONFlag(cmd, &opts.JSONOutput)

	return cmd
}

func viewRun(opts *viewOptions) error {
	repo, err := opts.Factory.BaseRepo()
	if err != nil {
		return err
	}

	index, err := strconv.ParseInt(opts.Number, 10, 64)
	if err != nil {
		return cmdutil.FlagErrorf("invalid issue number: %s", opts.Number)
	}

	client, err := opts.Factory.ClientForRepo(repo)
	if err != nil {
		return err
	}

	issue, _, err := client.GetIssue(repo.Owner, repo.Name, index)
	if err != nil {
		return fmt.Errorf("getting issue: %w", err)
	}

	if opts.Web {
		return cmdutil.OpenInBrowser(issue.HTMLURL)
	}

	if opts.JSONOutput {
		result := map[string]any{
			"issue": issue,
		}
		if opts.Comments {
			comments, _, err := client.ListIssueComments(repo.Owner, repo.Name, index, forgejo.ListIssueCommentOptions{})
			if err != nil {
				return fmt.Errorf("listing comments: %w", err)
			}
			result["comments"] = comments
		}
		if opts.ShowTimeline {
			events, err := opts.Factory.Timeline(repo, index)
			if err != nil {
				return err
			}
			result["timeline"] = events
		}
		if opts.Download {
			paths, dir, err := downloadIssueAttachments(opts.Factory, client, repo, index, opts.DownloadDir)
			if err != nil {
				return err
			}
			result["downloaded"] = map[string]any{
				"dir":   dir,
				"files": paths,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Text output
	fmt.Fprintf(os.Stdout, "%s #%d\n", issue.Title, issue.Index)
	fmt.Fprintf(os.Stdout, "State: %s\n", issue.State)
	if issue.Poster != nil {
		fmt.Fprintf(os.Stdout, "Author: %s\n", issue.Poster.UserName)
	}

	if len(issue.Labels) > 0 {
		var names []string
		for _, l := range issue.Labels {
			names = append(names, l.Name)
		}
		fmt.Fprintf(os.Stdout, "Labels: %s\n", strings.Join(names, ", "))
	}

	if len(issue.Assignees) > 0 {
		var names []string
		for _, a := range issue.Assignees {
			names = append(names, a.UserName)
		}
		fmt.Fprintf(os.Stdout, "Assignees: %s\n", strings.Join(names, ", "))
	}

	if issue.Milestone != nil {
		fmt.Fprintf(os.Stdout, "Milestone: %s\n", issue.Milestone.Title)
	}

	fmt.Fprintf(os.Stdout, "Created: %s\n", output.RelativeTimeStr(issue.Created))

	if issue.Body != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", output.RenderMarkdown(issue.Body))
	}

	fmt.Fprintf(os.Stdout, "\nView this issue on the web: %s\n", issue.HTMLURL)

	if opts.Comments {
		comments, _, err := client.ListIssueComments(repo.Owner, repo.Name, index, forgejo.ListIssueCommentOptions{})
		if err != nil {
			return fmt.Errorf("listing comments: %w", err)
		}
		if len(comments) > 0 {
			fmt.Fprintf(os.Stdout, "\n--- Comments (%d) ---\n", len(comments))
			for _, c := range comments {
				author := "unknown"
				if c.Poster != nil {
					author = c.Poster.UserName
				}
				fmt.Fprintf(
					os.Stdout, "\n%s commented %s:\n%s\n",
					author,
					output.RelativeTimeStr(c.Created),
					output.RenderMarkdown(c.Body),
				)
			}
		}
	}

	if opts.ShowTimeline {
		events, err := opts.Factory.Timeline(repo, index)
		if err != nil {
			return err
		}
		if lines := cmdutil.TimelineLines(events, cmdutil.SubjectIssue); len(lines) > 0 {
			fmt.Fprintf(os.Stdout, "\n--- Timeline (%d) ---\n", len(lines))
			for _, l := range lines {
				fmt.Fprintf(os.Stdout, "%s\n", l)
			}
		}
	}

	if opts.Download {
		paths, dir, err := downloadIssueAttachments(opts.Factory, client, repo, index, opts.DownloadDir)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			fmt.Fprintf(os.Stdout, "\nNo attachments on this issue.\n")
		} else {
			fmt.Fprintf(os.Stdout, "\nDownloaded %d attachment(s) to %s:\n", len(paths), dir)
			for _, p := range paths {
				fmt.Fprintf(os.Stdout, "  %s\n", p)
			}
		}
	}

	return nil
}

// downloadIssueAttachments fetches attachments for an issue (including its
// comments) and writes them to destDir (or ./issue-<index> if destDir is
// empty). Comment attachments are prefixed "comment-<id>-" to avoid collisions
// and to identify their source. Returns the written paths and the directory
// used.
func downloadIssueAttachments(f *cmdutil.Factory, client *forgejo.Client, repo cmdutil.Repo, index int64, destDir string) ([]string, string, error) {
	cfg, err := f.Config()
	if err != nil {
		return nil, "", err
	}
	token, err := cfg.TokenForHost(repo.Host)
	if err != nil {
		return nil, "", err
	}

	type item struct {
		url      string
		destName string
	}
	var items []item

	issueAtts, err := listAttachments(repo, fmt.Sprintf("issues/%d/assets", index), token)
	if err != nil {
		return nil, "", fmt.Errorf("listing issue attachments: %w", err)
	}
	for _, a := range issueAtts {
		items = append(items, item{url: a.DownloadURL, destName: filepath.Base(a.Name)})
	}

	comments, _, err := client.ListIssueComments(repo.Owner, repo.Name, index, forgejo.ListIssueCommentOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("listing comments: %w", err)
	}
	for _, c := range comments {
		catts, err := listAttachments(repo, fmt.Sprintf("issues/comments/%d/assets", c.ID), token)
		if err != nil {
			return nil, "", fmt.Errorf("listing comment %d attachments: %w", c.ID, err)
		}
		for _, a := range catts {
			items = append(items, item{
				url:      a.DownloadURL,
				destName: fmt.Sprintf("comment-%d-%s", c.ID, filepath.Base(a.Name)),
			})
		}
	}

	if len(items) == 0 {
		return nil, destDir, nil
	}

	dir := destDir
	if dir == "" {
		dir = fmt.Sprintf("issue-%d", index)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", fmt.Errorf("creating directory: %w", err)
	}

	paths := make([]string, 0, len(items))
	for _, it := range items {
		dest := filepath.Join(dir, it.destName)
		if err := downloadAttachment(it.url, token, dest); err != nil {
			return nil, "", fmt.Errorf("downloading %s: %w", it.destName, err)
		}
		fmt.Fprintf(os.Stderr, "✓ Downloaded %s\n", it.destName)
		paths = append(paths, dest)
	}
	return paths, dir, nil
}

func listAttachments(repo cmdutil.Repo, subpath, token string) ([]*forgejo.Attachment, error) {
	scheme := "https"
	if os.Getenv("FJ_INSECURE") != "" {
		scheme = "http"
	}
	url := fmt.Sprintf("%s://%s/api/v1/repos/%s/%s/%s",
		scheme, repo.Host, repo.Owner, repo.Name, subpath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var atts []*forgejo.Attachment
	if err := json.NewDecoder(resp.Body).Decode(&atts); err != nil {
		return nil, err
	}
	return atts, nil
}

func downloadAttachment(url, token, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
