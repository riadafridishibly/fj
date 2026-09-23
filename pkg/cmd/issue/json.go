package issue

import (
	"fmt"
	"net/url"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// issueFields are gh's issue JSON fields that Forgejo can fill. blockedBy
// and blocking could come from Forgejo's issue dependencies; not done yet.
var issueFields = []string{
	"assignees", "author", "body", "closed", "closedAt", "comments", "createdAt",
	"id", "isPinned", "labels", "milestone", "number", "state", "title", "updatedAt", "url",
}

// apiIssue is forgejo.Issue plus pin_order, which the SDK does not decode.
type apiIssue struct {
	forgejo.Issue
	PinOrder int64 `json:"pin_order"`
}

func issuePath(repo cmdutil.Repo, format string, args ...any) string {
	return fmt.Sprintf("/repos/%s/%s", url.PathEscape(repo.Owner), url.PathEscape(repo.Name)) +
		fmt.Sprintf(format, args...)
}

// getIssue fetches an issue with its pin_order.
func getIssue(f *cmdutil.Factory, repo cmdutil.Repo, index int64) (*apiIssue, error) {
	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	issue := new(apiIssue)
	if err := client.GetJSON(issuePath(repo, "/issues/%d", index), issue); err != nil {
		return nil, fmt.Errorf("getting issue: %w", err)
	}
	return issue, nil
}

// issueJSON returns issue keyed by gh field name. Comments cost a request,
// so they are listed only when the comments field was asked for.
func issueJSON(f *cmdutil.Factory, repo cmdutil.Repo, issue *apiIssue, j *cmdutil.JSONFlags) (map[string]any, error) {
	author := cmdutil.JSONUser(issue.Poster)
	if author != nil {
		author["is_bot"] = false
	}
	m := map[string]any{
		"assignees": cmdutil.JSONUsers(issue.Assignees),
		"author":    author,
		"body":      issue.Body,
		"closed":    issue.State == forgejo.StateClosed,
		"closedAt":  cmdutil.JSONTime(issue.Closed),
		"createdAt": cmdutil.JSONTime(&issue.Created),
		"id":        issue.ID,
		"isPinned":  issue.PinOrder > 0,
		"labels":    cmdutil.JSONLabels(issue.Labels),
		"milestone": cmdutil.JSONMilestone(issue.Milestone),
		"number":    issue.Index,
		"state":     strings.ToUpper(string(issue.State)),
		"title":     issue.Title,
		"updatedAt": cmdutil.JSONTime(&issue.Updated),
		"url":       issue.HTMLURL,
	}
	if !j.Has("comments") {
		return m, nil
	}

	client, err := f.APIClient(repo)
	if err != nil {
		return nil, err
	}
	var comments []*forgejo.Comment
	if err := client.GetJSON(issuePath(repo, "/issues/%d/comments", issue.Index), &comments); err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}
	out := make([]map[string]any, len(comments))
	for i, c := range comments {
		var login string
		if c.Poster != nil {
			login = c.Poster.UserName
		}
		out[i] = map[string]any{
			"id": c.ID, "author": map[string]any{"login": login}, "body": c.Body,
			"createdAt": cmdutil.JSONTime(&c.Created), "url": c.HTMLURL,
		}
	}
	m["comments"] = out
	return m, nil
}
