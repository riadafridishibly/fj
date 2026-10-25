package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// ListOrgLabels lists one page of an organization's labels. The SDK covers
// repository labels only. The response is returned for its X-Total-Count
// header, and for its status when the call fails: a user, not an
// organization, gets a 404.
func (c *Client) ListOrgLabels(org string, opt forgejo.ListOptions) ([]*forgejo.Label, *http.Response, error) {
	path := fmt.Sprintf("/orgs/%s/labels?page=%d&limit=%d", url.PathEscape(org), opt.Page, opt.PageSize)
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, resp, statusError(resp)
	}
	var labels []*forgejo.Label
	if err := json.NewDecoder(resp.Body).Decode(&labels); err != nil {
		return nil, resp, err
	}
	return labels, resp, nil
}
