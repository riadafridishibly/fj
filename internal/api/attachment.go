package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

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
		repoPath(owner, repo, "/issues/%d/assets", index),
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
		repoPath(owner, repo, "/issues/comments/%d/assets", commentID),
		file, filename)
}

// createAttachment posts file to path as a multipart form under the field
// name "attachment", which is the only name either assets endpoint accepts.
// The body is buffered in memory before the request goes out, matching what
// the SDK does for release attachments: it gives the request a Content-Length
// rather than a chunked body, which is what the assets endpoints expect.
func (c *Client) createAttachment(path string, file io.Reader, filename string) (*forgejo.Attachment, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}
	if err := checkFilename(filename); err != nil {
		return nil, err
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

	out := new(forgejo.Attachment)
	if err := c.send(req, out); err != nil {
		return nil, err
	}
	return out, nil
}

// checkFilename rejects names that cannot be carried safely in a multipart
// part header. mime/multipart escapes only backslash and double quote, so a
// carriage return or newline in a filename is written into the part's
// Content-Disposition verbatim and terminates the header — letting the name
// inject MIME headers of its own.
func checkFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if i := strings.IndexFunc(filename, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}); i >= 0 {
		return fmt.Errorf("filename contains a control character at byte %d: %q", i, filename)
	}
	return nil
}
