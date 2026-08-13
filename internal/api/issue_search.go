package api

import (
	"fmt"
	"net/http"
	"net/url"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// searchIssuesPageSize and searchIssuesMaxPages bound the search. The
// endpoint cannot filter by repository, so a caller narrowing to one
// discards most of every page and its matches can sit well past the first —
// paging is not optional here. The cap still keeps one command from fanning
// out into dozens of requests.
const (
	searchIssuesPageSize = 50
	searchIssuesMaxPages = 10
)

// SearchIssuesOptions filters GET /repos/issues/search. Only the fields fj
// uses are modelled.
//
// Unlike the per-repository issue list, this endpoint selects by the
// caller's *relationship* to an issue rather than by username, which is the
// only way to ask "which pull requests want a review from me". The SDK's
// ListIssueOption has no field for it and sends the username-based
// parameters here instead, where the server ignores them.
type SearchIssuesOptions struct {
	// ListOptions sizes a single page. SearchIssues drives Page itself and
	// reads to the end of the results, so callers set only PageSize, and
	// only when they want pages smaller than searchIssuesPageSize.
	forgejo.ListOptions
	// Type is "issues" or "pulls"; empty means both.
	Type string
	// State is "open", "closed", or "all".
	State string
	// Owner narrows the search to a user or organization. The endpoint has
	// no repository filter at all — callers wanting one repository filter
	// the results themselves.
	Owner           string
	ReviewRequested bool
}

func (opt *SearchIssuesOptions) QueryEncode() string {
	query := make(url.Values)
	if opt.Page > 0 {
		query.Add("page", fmt.Sprintf("%d", opt.Page))
	}
	if opt.PageSize > 0 {
		query.Add("limit", fmt.Sprintf("%d", opt.PageSize))
	}
	if opt.Type != "" {
		query.Add("type", opt.Type)
	}
	if opt.State != "" {
		query.Add("state", opt.State)
	}
	if opt.Owner != "" {
		query.Add("owner", opt.Owner)
	}
	if opt.ReviewRequested {
		query.Add("review_requested", "true")
	}
	return query.Encode()
}

// SearchIssues lists issues or pull requests across repositories, filtered
// by the authenticated user's relationship to them. It pages until the
// server returns a short page, or until searchIssuesMaxPages pages have
// been read.
func (c *Client) SearchIssues(opt SearchIssuesOptions) ([]*forgejo.Issue, error) {
	if opt.PageSize <= 0 {
		opt.PageSize = searchIssuesPageSize
	}

	var all []*forgejo.Issue
	for page := 1; page <= searchIssuesMaxPages; page++ {
		opt.Page = page

		var issues []*forgejo.Issue
		if err := c.do(http.MethodGet, "/repos/issues/search?"+opt.QueryEncode(), nil, &issues); err != nil {
			return nil, err
		}
		all = append(all, issues...)
		if len(issues) < opt.PageSize {
			break
		}
	}
	return all, nil
}
