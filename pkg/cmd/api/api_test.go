package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/config"
)

// TestBuildBody pins gh's field syntax: typed -F values, nested objects,
// array appends, a bare key[] for an empty array, and consecutive
// key[][name] fields grouping into one array element until a name repeats.
func TestBuildBody(t *testing.T) {
	file := filepath.Join(t.TempDir(), "content.txt")
	if err := os.WriteFile(file, []byte("from file"), 0o644); err != nil {
		t.Fatal(err)
	}

	raw := []string{
		"title=42",
		"labels[]=bug",
		"labels[]=docs",
		"assignees[]",
		"props[][name]=env",
		"props[][allowed][]=staging",
		"props[][allowed][]=production",
		"props[][name]=tier",
		"meta[owner]={owner}",
	}
	magic := []string{
		"count=42",
		"draft=true",
		"milestone=null",
		"meta[repo]={repo}",
		"content=@" + file,
	}
	fill := func(s string) string {
		return strings.NewReplacer("{owner}", "me", "{repo}", "fj").Replace(s)
	}

	fields, err := parseFields(raw, magic, fill)
	if err != nil {
		t.Fatalf("parseFields() error = %v", err)
	}
	body, err := buildBody(fields)
	if err != nil {
		t.Fatalf("buildBody() error = %v", err)
	}

	got, _ := json.Marshal(body)
	var gotAny, wantAny any
	json.Unmarshal(got, &gotAny)
	json.Unmarshal([]byte(`{
		"title": "42",
		"labels": ["bug", "docs"],
		"assignees": [],
		"props": [
			{"name": "env", "allowed": ["staging", "production"]},
			{"name": "tier"}
		],
		"meta": {"owner": "{owner}", "repo": "fj"},
		"count": 42,
		"draft": true,
		"milestone": null,
		"content": "from file"
	}`), &wantAny)
	if !reflect.DeepEqual(gotAny, wantAny) {
		t.Errorf("body =\n%s", got)
	}
}

func TestParseFieldsErrors(t *testing.T) {
	for _, tc := range []struct{ field, want string }{
		{"title", `requires a value separated by an '=' sign`},
		{"[x]=1", `invalid key`},
		{"a[b=1", `invalid key`},
	} {
		fields, err := parseFields([]string{tc.field}, nil, nil)
		if err == nil {
			_, err = buildBody(fields)
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: error = %v, want it to contain %q", tc.field, err, tc.want)
		}
	}

	fields, _ := parseFields([]string{"a=1", "a[b]=2"}, nil, nil)
	if _, err := buildBody(fields); err == nil {
		t.Error("a=1 then a[b]=2: want a conflict error")
	}
}

// TestNextPage covers the two ways a naive Link parser goes wrong: a comma
// inside a URL's query, and a ROOT_URL that names a different host than the
// one fj reached.
func TestNextPage(t *testing.T) {
	link := `<http://forgejo.example.com:3000/api/v1/repos/o/r/issues?labels=bug,docs&page=3>; rel="next",` +
		`<http://forgejo.example.com:3000/api/v1/repos/o/r/issues?labels=bug,docs&page=9>; rel="last"`
	if got, want := nextPage(link), "repos/o/r/issues?labels=bug,docs&page=3"; got != want {
		t.Errorf("nextPage() = %q, want %q", got, want)
	}
	if got := nextPage(`<http://h/api/v1/x?page=1>; rel="first"`); got != "" {
		t.Errorf("nextPage() on the last page = %q, want empty", got)
	}
}

// fakeForgejo serves three pages of labels, a 404, and an echo endpoint
// that returns the request it saw. Its Link headers name a different host,
// as a Forgejo behind a proxy does.
func fakeForgejo(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "token sekrit" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		switch r.URL.Path {
		case "/api/v1/repos/me/fj/labels":
			page := r.URL.Query().Get("page")
			if page == "" {
				page = "1"
			}
			if page != "3" {
				next := map[string]string{"1": "2", "2": "3"}[page]
				w.Header().Set("Link", fmt.Sprintf(`<http://forgejo.example.com/api/v1/repos/me/fj/labels?limit=1&page=%s>; rel="next"`, next))
			}
			fmt.Fprintf(w, `[{"id":%s000000000000000001,"name":"<l%s>"}]`+"\n", page, page)
		case "/api/v1/echo":
			body, _ := io.ReadAll(r.Body)
			json.NewEncoder(w).Encode(map[string]string{
				"method":       r.Method,
				"query":        r.URL.RawQuery,
				"content_type": r.Header.Get("Content-Type"),
				"body":         string(body),
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"message":"The target couldn't be found.","url":"x"}`)
		}
	}))
}

// runAPI runs fj api with srv as the only configured host and -R me/fj.
func runAPI(t *testing.T, srv *httptest.Server, args ...string) (string, error) {
	t.Helper()
	f := factory(strings.TrimPrefix(srv.URL, "http://"))
	f.RepoOverride = "me/fj"
	return runAPIWith(t, f, args...)
}

// factory builds a Factory whose config holds hosts, each with the token
// the fake server expects.
func factory(hosts ...string) *cmdutil.Factory {
	cfg := &config.Config{Hosts: map[string]*config.HostConfig{}}
	for _, h := range hosts {
		cfg.Hosts[h] = &config.HostConfig{Hostname: h, Token: "sekrit", User: "me"}
	}
	return &cmdutil.Factory{Config: func() (*config.Config, error) { return cfg, nil }}
}

func runAPIWith(t *testing.T, f *cmdutil.Factory, args ...string) (string, error) {
	t.Helper()
	t.Setenv("FJ_INSECURE", "1")
	t.Setenv("FJ_TOKEN", "")
	t.Setenv("FJ_HOST", "")
	t.Chdir(t.TempDir())
	cmd := NewCmdAPI(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestPaginateMergesArrays(t *testing.T) {
	srv := fakeForgejo(t)
	defer srv.Close()

	out, err := runAPI(t, srv, "--paginate", "repos/{owner}/{repo}/labels?limit=1")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	// One array, element bytes untouched: the IDs past 2^53 and the
	// unescaped angle brackets are exactly as served.
	want := `[{"id":1000000000000000001,"name":"<l1>"},{"id":2000000000000000001,"name":"<l2>"},{"id":3000000000000000001,"name":"<l3>"}]` + "\n"
	if out != want {
		t.Errorf("output =\n%s\nwant\n%s", out, want)
	}

	out, err = runAPI(t, srv, "--paginate", "--jq", ".[].id", "repos/{owner}/{repo}/labels")
	if err != nil {
		t.Fatalf("--jq error = %v", err)
	}
	if want := "1000000000000000001\n2000000000000000001\n3000000000000000001\n"; out != want {
		t.Errorf("--jq output = %q, want %q", out, want)
	}

	out, err = runAPI(t, srv, "--paginate", "--slurp", "repos/{owner}/{repo}/labels")
	if err != nil {
		t.Fatalf("--slurp error = %v", err)
	}
	var pages [][]any
	if err := json.Unmarshal([]byte(out), &pages); err != nil || len(pages) != 3 {
		t.Errorf("--slurp output = %s, want 3 pages (err %v)", out, err)
	}
}

func TestFieldsPlacement(t *testing.T) {
	srv := fakeForgejo(t)
	defer srv.Close()

	echo := func(args ...string) map[string]string {
		t.Helper()
		out, err := runAPI(t, srv, append([]string{"echo"}, args...)...)
		if err != nil {
			t.Fatalf("fj api echo %v: %v", args, err)
		}
		var got map[string]string
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, out)
		}
		return got
	}

	got := echo("-f", "title=hi", "-F", "n=3")
	if got["method"] != "POST" || got["content_type"] != "application/json" || got["body"] != `{"n":3,"title":"hi"}` {
		t.Errorf("fields without -X: %v", got)
	}

	got = echo("-X", "get", "-f", "q=a b", "-F", "limit=5")
	if got["method"] != "GET" || got["query"] != "limit=5&q=a+b" || got["body"] != "" {
		t.Errorf("fields with -X GET: %v", got)
	}

	input := filepath.Join(t.TempDir(), "body.json")
	os.WriteFile(input, []byte(`{"raw":true}`), 0o644)
	got = echo("--input", input, "-f", "dry=1", "-H", "Content-Type: text/plain")
	if got["method"] != "POST" || got["body"] != `{"raw":true}` || got["query"] != "dry=1" || got["content_type"] != "text/plain" {
		t.Errorf("--input with a field and -H: %v", got)
	}
}

// TestHTTPError pins gh's error contract: the body is printed unfiltered,
// even with --jq, and the error names the server's message and status.
func TestHTTPError(t *testing.T) {
	srv := fakeForgejo(t)
	defer srv.Close()

	out, err := runAPI(t, srv, "--jq", ".message", "repos/{owner}/{repo}/nope")
	if err == nil || err.Error() != "The target couldn't be found. (HTTP 404)" {
		t.Errorf("error = %v", err)
	}
	if !strings.HasPrefix(out, `{"message":`) {
		t.Errorf("output = %q, want the raw body", out)
	}
}

func TestAbsoluteURLHost(t *testing.T) {
	srv := fakeForgejo(t)
	defer srv.Close()

	if _, err := runAPI(t, srv, srv.URL+"/api/v1/echo"); err != nil {
		t.Errorf("URL on the logged-in host: %v", err)
	}
	_, err := runAPI(t, srv, "http://forgejo.example.com/api/v1/user")
	if err == nil || !strings.Contains(err.Error(), "not logged in to forgejo.example.com") {
		t.Errorf("URL on another host: error = %v", err)
	}
}

// TestHostWithoutPlaceholders pins the resolver as the fallback: with two
// hosts and nothing to choose between them, the request is refused rather
// than sent to one of them at random.
func TestHostWithoutPlaceholders(t *testing.T) {
	srv := fakeForgejo(t)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	_, err := runAPIWith(t, factory(host, "forgejo.example.com"), "echo")
	if err == nil || !strings.Contains(err.Error(), "multiple hosts configured") {
		t.Errorf("two hosts: error = %v, want the resolver's", err)
	}

	if _, err := runAPIWith(t, factory(host, "forgejo.example.com"), "--hostname", host, "echo"); err != nil {
		t.Errorf("--hostname: %v", err)
	}
}

// TestAbsoluteURLOnFJHost covers CI without a config file: a full URL on
// the FJ_HOST host is allowed, and FJ_TOKEN goes with it.
func TestAbsoluteURLOnFJHost(t *testing.T) {
	srv := fakeForgejo(t)
	defer srv.Close()

	f := factory()
	t.Setenv("FJ_HOST", strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("FJ_TOKEN", "sekrit")
	cmd := NewCmdAPI(f)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{srv.URL + "/api/v1/echo"})
	t.Setenv("FJ_INSECURE", "1")
	if err := cmd.Execute(); err != nil {
		t.Errorf("full URL on FJ_HOST: %v", err)
	}
}
