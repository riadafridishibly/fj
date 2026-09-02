package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// CreateIssueAttachment uploads a file and attaches it to an issue. The
// returned attachment's DownloadURL is the browser-facing link to the file,
// which is what a markdown body embeds.
//
// filename is the name Forgejo stores and displays; it does not have to
// match the path the bytes were read from. Pull requests share the issue
// index space, so passing a pull request's index attaches to that pull
// request.
//
// The SDK covers release attachments only, so there is no SDK equivalent.
func (c *Client) CreateIssueAttachment(owner, repo string, index int64, file io.Reader, filename string) (*forgejo.Attachment, error) {
	return c.createAttachment(
		fmt.Sprintf("/repos/%s/%s/issues/%d/assets", owner, repo, index),
		file, filename)
}

// CreateIssueCommentAttachment uploads a file and attaches it to a single
// comment rather than to the issue itself. Comments are addressed by their
// own ID, not by the index of the issue they belong to, so the owner and
// repo are all the endpoint needs beyond the comment.
//
// The SDK covers release attachments only, so there is no SDK equivalent.
func (c *Client) CreateIssueCommentAttachment(owner, repo string, commentID int64, file io.Reader, filename string) (*forgejo.Attachment, error) {
	return c.createAttachment(
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets", owner, repo, commentID),
		file, filename)
}

// createAttachment posts file to path as a multipart form under the field
// name "attachment", which is the only name either assets endpoint accepts.
// The body is buffered in memory before the request goes out, matching what
// the SDK does for release attachments.
func (c *Client) createAttachment(path string, file io.Reader, filename string) (*forgejo.Attachment, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("attachment", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := c.newBodyRequest(http.MethodPost, path, body, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, statusError(resp)
	}

	out := new(forgejo.Attachment)
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}
