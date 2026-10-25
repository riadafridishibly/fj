package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// MergePullRequest preserves error responses that the SDK's status-only
// implementation discards. A 201 response schedules an automatic merge;
// only 200 means the pull request was merged immediately.
func (c *Client) MergePullRequest(owner, repo string, index int64, opt forgejo.MergePullRequestOption) (bool, error) {
	body, err := json.Marshal(opt)
	if err != nil {
		return false, err
	}
	resp, err := c.Raw(http.MethodPost, repoPath(owner, repo, "/pulls/%d/merge", index),
		bytes.NewReader(body), "application/json", nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false, statusError(resp)
	}
	return resp.StatusCode == http.StatusOK, nil
}
