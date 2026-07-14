// Package api provides SDK-style wrappers for Forgejo API endpoints that
// the upstream forgejo-sdk does not cover. Methods mirror the SDK's shape
// (owner/repo/index arguments, SDK types for payloads) so call sites read
// the same whether they use the SDK or this package.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Client calls Forgejo API endpoints missing from the SDK. It is safe for
// concurrent use.
type Client struct {
	baseURL string // scheme://host, no trailing slash
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: baseURL, token: token, http: httpClient}
}

// CreatePullReviewComment adds a single inline comment to an existing
// review. Posting to the review that holds another comment, at that
// comment's path and position, threads the two together in the Forgejo UI —
// this is the only reply mechanism the API offers (no in_reply_to field).
func (c *Client) CreatePullReviewComment(owner, repo string, index, reviewID int64, opt forgejo.CreatePullReviewComment) (*forgejo.PullReviewComment, error) {
	if opt.Path == "" || opt.Body == "" {
		return nil, fmt.Errorf("path and body are required")
	}
	out := new(forgejo.PullReviewComment)
	err := c.do(http.MethodPost,
		fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews/%d/comments", owner, repo, index, reviewID),
		opt, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetPullReviewComment fetches a single inline review comment.
func (c *Client) GetPullReviewComment(owner, repo string, index, reviewID, commentID int64) (*forgejo.PullReviewComment, error) {
	out := new(forgejo.PullReviewComment)
	err := c.do(http.MethodGet,
		fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews/%d/comments/%d", owner, repo, index, reviewID, commentID),
		nil, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeletePullReviewComment deletes a single inline review comment. The
// regular issue-comment delete endpoint does not accept code comments;
// this dedicated endpoint is the only way to delete one.
func (c *Client) DeletePullReviewComment(owner, repo string, index, reviewID, commentID int64) error {
	return c.do(http.MethodDelete,
		fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews/%d/comments/%d", owner, repo, index, reviewID, commentID),
		nil, nil)
}

// Get performs a raw authenticated GET against /api/v1<path> and returns
// the response. Caller owns closing resp.Body. Escape hatch for endpoints
// without a typed wrapper.
func (c *Client) Get(path string) (*http.Response, error) {
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

func (c *Client) newRequest(method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, c.baseURL+"/api/v1"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// do runs a request and decodes the JSON response into out (skipped when
// out is nil). Non-2xx responses become errors carrying the server message.
func (c *Client) do(method, path string, body, out any) error {
	req, err := c.newRequest(method, path, body)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return statusError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func statusError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Message != "" {
		return fmt.Errorf("%s: %s", resp.Status, payload.Message)
	}
	return fmt.Errorf("unexpected status: %s", resp.Status)
}
