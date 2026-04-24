package debug

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/riadafridishibly/fj/internal/output"
)

// maxBodyLog bounds how many bytes of a request or response body we
// print at DEBUG=3. Bodies larger than this are truncated in the log
// but passed through to the caller in full.
const maxBodyLog = 64 * 1024

var truncatedMarker = []byte("…(truncated)")

// Transport wraps an http.RoundTripper and logs each request. See the
// package doc for the level semantics. A nil Base falls back to
// http.DefaultTransport.
type Transport struct {
	Base http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if !Enabled(2) {
		return base.RoundTrip(req)
	}

	label := methodPath(req)

	if Enabled(3) {
		if body, ct := snapshotBody(&req.Body, req.Header.Get("Content-Type")); body != nil {
			Body(3, label+" request body:", body, ct)
		}
	}

	Logf(2, "-> %s", label)
	start := time.Now()
	resp, err := base.RoundTrip(req)
	dur := time.Since(start)

	switch {
	case err != nil:
		Logf(2, "<- %s error=%v %s", label, err, paint(output.Dim, fmt.Sprintf("(%s)", dur)))
	case resp != nil:
		status := paint(statusColor(resp.StatusCode), resp.Status)
		extra := ""
		if Enabled(3) {
			if v := resp.Header.Get("X-Total-Count"); v != "" {
				extra = " " + paint(output.Dim, "total="+v)
			}
		}
		Logf(2, "<- %s %s%s %s", label, status, extra, paint(output.Dim, fmt.Sprintf("(%s)", dur)))
		if Enabled(3) {
			if body, ct := snapshotBody(&resp.Body, resp.Header.Get("Content-Type")); body != nil {
				Body(3, label+" response body:", body, ct)
			}
		}
	}
	return resp, err
}

// methodPath returns "METHOD /path" for logs, colored by method verb, and
// appends the query string at DEBUG=3.
func methodPath(req *http.Request) string {
	label := paint(methodColor(req.Method), req.Method) + " " + req.URL.Path
	if Enabled(3) && req.URL.RawQuery != "" {
		label += "?" + req.URL.RawQuery
	}
	return label
}

// WrapClient returns an *http.Client whose transport logs to Logf. If c is
// nil, a fresh client is returned. The original client's transport (if any)
// is used as the base, so existing behavior is preserved.
func WrapClient(c *http.Client) *http.Client {
	if c == nil {
		return &http.Client{Transport: &Transport{}}
	}
	return &http.Client{
		Transport:     &Transport{Base: c.Transport},
		CheckRedirect: c.CheckRedirect,
		Jar:           c.Jar,
		Timeout:       c.Timeout,
	}
}

// snapshotBody reads up to maxBodyLog+1 bytes of bodyPtr for logging and
// restores bodyPtr so the downstream consumer sees the full, unmodified
// body. Returns (nil, "") when there is nothing loggable (nil body, empty,
// or binary content). The returned contentType is echoed back so callers
// can hand it to Body.
func snapshotBody(bodyPtr *io.ReadCloser, contentType string) ([]byte, string) {
	body := *bodyPtr
	if body == nil || body == http.NoBody {
		return nil, ""
	}
	if isBinary(contentType) {
		return nil, ""
	}

	var buf bytes.Buffer
	_, err := io.CopyN(&buf, body, maxBodyLog+1)
	data := buf.Bytes()

	switch {
	case err == io.EOF:
		body.Close()
		*bodyPtr = io.NopCloser(bytes.NewReader(data))
		return data, contentType
	case err != nil:
		body.Close()
		*bodyPtr = io.NopCloser(bytes.NewReader(data))
		return fmt.Appendf(data, " <read error: %v>", err), contentType
	default:
		// The extra +1 byte past maxBodyLog means more may follow — tee the
		// buffered bytes ahead of the unread tail so the caller sees the
		// full body.
		tail := data[maxBodyLog:]
		*bodyPtr = &multiReadCloser{
			Reader: io.MultiReader(bytes.NewReader(tail), body),
			closer: body,
		}
		return append(data[:maxBodyLog:maxBodyLog], truncatedMarker...), contentType
	}
}

type multiReadCloser struct {
	io.Reader
	closer io.Closer
}

func (m *multiReadCloser) Close() error { return m.closer.Close() }
