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
	"net/url"

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
		repoPath(owner, repo, "/pulls/%d/reviews/%d/comments", index, reviewID),
		opt, out)
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
		repoPath(owner, repo, "/pulls/%d/reviews/%d/comments/%d", index, reviewID, commentID),
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

// repoPath builds a path under /repos/<owner>/<repo>, escaping both
// segments and formatting the rest. Escaping is not cosmetic: an owner or
// repo carrying "?", "#" or a ".." segment would otherwise retarget the
// request at a different endpoint, since http.NewRequest splits the query
// off the path and does not resolve dot segments. The SDK escapes every
// owner/repo the same way before building a path.
func repoPath(owner, repo, format string, args ...any) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) +
		fmt.Sprintf(format, args...)
}

func (c *Client) newRequest(method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	var contentType string
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
		contentType = "application/json"
	}
	return c.newBodyRequest(method, path, reader, contentType)
}

// newBodyRequest builds an authenticated request against /api/v1<path> whose
// body is sent verbatim. newRequest wraps it for JSON payloads; callers whose
// payload is not JSON — a multipart upload, whose Content-Type carries a
// generated boundary — build the body themselves and use this directly.
// An empty contentType sets no header, as a bodiless request wants.
func (c *Client) newBodyRequest(method, path string, body io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequest(method, c.baseURL+"/api/v1"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// do builds a request with a JSON body and hands it to send.
func (c *Client) do(method, path string, body, out any) error {
	req, err := c.newRequest(method, path, body)
	if err != nil {
		return err
	}
	return c.send(req, out)
}

// send runs a request and decodes the JSON response into out (skipped when
// out is nil). Non-2xx responses become errors carrying the server message.
// Callers that build their own request — a multipart upload — use this
// directly, so every response this package handles takes one path.
func (c *Client) send(req *http.Request, out any) error {
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
