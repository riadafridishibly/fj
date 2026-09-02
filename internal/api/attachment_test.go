package api

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const attachmentResponse = `{
	"id": 7,
	"name": "login.png",
	"size": 13,
	"download_count": 0,
	"created_at": "2026-07-17T10:00:00Z",
	"uuid": "0f8ad0f2-1f19-4a3f-9c4c-b09e2ff0f6a1",
	"browser_download_url": "https://code.example.com/attachments/0f8ad0f2-1f19-4a3f-9c4c-b09e2ff0f6a1"
}`

// upload captures what the server saw for one attachment request.
type upload struct {
	method      string
	path        string
	auth        string
	accept      string
	contentType string
	field       string
	filename    string
	content     string
}

// attachmentServer answers one upload with attachmentResponse and records
// the request, including the parsed multipart part.
func attachmentServer(t *testing.T, got *upload) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.auth = r.Header.Get("Authorization")
		got.accept = r.Header.Get("Accept")
		got.contentType = r.Header.Get("Content-Type")

		_, params, err := mime.ParseMediaType(got.contentType)
		if err != nil {
			t.Errorf("parsing Content-Type %q: %v", got.contentType, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		part, err := multipart.NewReader(r.Body, params["boundary"]).NextPart()
		if err != nil {
			t.Errorf("reading multipart body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		got.field, got.filename = part.FormName(), part.FileName()
		content, err := io.ReadAll(part)
		if err != nil {
			t.Errorf("reading part content: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		got.content = string(content)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(attachmentResponse))
	}))
}

// checkUpload asserts everything the two endpoints share: the verb, the
// auth and accept headers, and a multipart body whose single part carries
// the file under the field name Forgejo requires.
func checkUpload(t *testing.T, got upload, wantPath, wantFilename, wantContent string) {
	t.Helper()
	if got.method != http.MethodPost {
		t.Errorf("method = %q, want %q", got.method, http.MethodPost)
	}
	if got.path != wantPath {
		t.Errorf("path = %q, want %q", got.path, wantPath)
	}
	if want := "token sekrit"; got.auth != want {
		t.Errorf("Authorization = %q, want %q", got.auth, want)
	}
	if want := "application/json"; got.accept != want {
		t.Errorf("Accept = %q, want %q", got.accept, want)
	}
	if !strings.HasPrefix(got.contentType, "multipart/form-data;") {
		t.Errorf("Content-Type = %q, want a multipart/form-data type", got.contentType)
	}
	if got.field != "attachment" {
		t.Errorf("form field = %q, want %q", got.field, "attachment")
	}
	if got.filename != wantFilename {
		t.Errorf("filename = %q, want %q", got.filename, wantFilename)
	}
	if got.content != wantContent {
		t.Errorf("uploaded content = %q, want %q", got.content, wantContent)
	}
}

func TestCreateIssueAttachment(t *testing.T) {
	var got upload
	srv := attachmentServer(t, &got)
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	att, err := client.CreateIssueAttachment("owner", "repo", 42,
		strings.NewReader("PNG bytes here"), "login.png")
	if err != nil {
		t.Fatalf("CreateIssueAttachment() error = %v", err)
	}

	checkUpload(t, got, "/api/v1/repos/owner/repo/issues/42/assets", "login.png", "PNG bytes here")

	if att.ID != 7 {
		t.Errorf("ID = %d, want 7", att.ID)
	}
	if att.Name != "login.png" {
		t.Errorf("Name = %q, want %q", att.Name, "login.png")
	}
	// The download URL is the whole point of the call: fj prints it so the
	// user can paste it into a markdown body.
	if want := "https://code.example.com/attachments/0f8ad0f2-1f19-4a3f-9c4c-b09e2ff0f6a1"; att.DownloadURL != want {
		t.Errorf("DownloadURL = %q, want %q", att.DownloadURL, want)
	}
	if want := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC); !att.Created.Equal(want) {
		t.Errorf("Created = %v, want %v", att.Created, want)
	}
}

func TestCreateIssueCommentAttachment(t *testing.T) {
	var got upload
	srv := attachmentServer(t, &got)
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	// Comments are addressed by their own ID, not by an issue index.
	if _, err := client.CreateIssueCommentAttachment("owner", "repo", 1234,
		strings.NewReader("log line\n"), "crash.log"); err != nil {
		t.Fatalf("CreateIssueCommentAttachment() error = %v", err)
	}

	checkUpload(t, got, "/api/v1/repos/owner/repo/issues/comments/1234/assets", "crash.log", "log line\n")
}

func TestCreateIssueAttachmentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte(`{"message":"file is too large"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	_, err := client.CreateIssueAttachment("owner", "repo", 42,
		strings.NewReader("too many bytes"), "huge.bin")
	if err == nil {
		t.Fatal("CreateIssueAttachment() error = nil, want an error carrying the server message")
	}
	if !strings.Contains(err.Error(), "file is too large") {
		t.Errorf("error = %q, want it to carry the server message", err)
	}
}

// TestCreateIssueAttachmentRejectsFilename covers the names that must never
// reach the wire: an empty one the endpoint rejects anyway, and control
// characters, which multipart does not escape and which would therefore write
// attacker-chosen headers into the part.
func TestCreateIssueAttachmentRejectsFilename(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request sent despite a rejected filename")
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	names := map[string]string{
		"empty":           "",
		"newline":         "note.txt\nContent-Type: text/html",
		"carriage return": "note.txt\r\nContent-Type: text/html",
		"nul":             "note\x00.txt",
		"delete":          "note\x7f.txt",
	}
	for label, name := range names {
		t.Run(label, func(t *testing.T) {
			if _, err := client.CreateIssueAttachment("owner", "repo", 42,
				strings.NewReader("bytes"), name); err == nil {
				t.Fatalf("CreateIssueAttachment(%q) error = nil, want a rejection", name)
			}
		})
	}
}

// TestCreateIssueAttachmentKeepsOrdinaryNames guards the check above from
// growing teeth it should not have: a quote or a space is legal in a filename
// and multipart already escapes what it needs to.
func TestCreateIssueAttachmentKeepsOrdinaryNames(t *testing.T) {
	var got upload
	srv := attachmentServer(t, &got)
	defer srv.Close()

	client := NewClient(srv.URL, "sekrit", srv.Client())
	name := `my "best" shot.png`
	if _, err := client.CreateIssueAttachment("owner", "repo", 42,
		strings.NewReader("bytes"), name); err != nil {
		t.Fatalf("CreateIssueAttachment(%q) error = %v", name, err)
	}
	if got.filename != name {
		t.Errorf("filename = %q, want %q", got.filename, name)
	}
}
