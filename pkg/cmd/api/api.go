package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	apiclient "github.com/riadafridishibly/fj/internal/api"
	"github.com/riadafridishibly/fj/internal/cmdutil"
	"github.com/riadafridishibly/fj/internal/debug"
	"github.com/riadafridishibly/fj/internal/git"
	"github.com/riadafridishibly/fj/internal/output"
)

type apiOptions struct {
	Factory *cmdutil.Factory
	Out     io.Writer

	Endpoint     string
	Method       string
	MethodPassed bool
	RawFields    []string
	MagicFields  []string
	Headers      []string
	Input        string
	Include      bool
	Silent       bool
	Verbose      bool
	Paginate     bool
	Slurp        bool
	JQ           string
}

func NewCmdAPI(f *cmdutil.Factory) *cobra.Command {
	opts := &apiOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "api <endpoint>",
		Short: "Make an authenticated Forgejo API request",
		Long: `Make an authenticated HTTP request to the Forgejo API and print the response.

The endpoint is a path under /api/v1, such as repos/{owner}/{repo}/issues, or
a full URL on the same host. Every server publishes its API reference at
https://<host>/swagger.v1.json.

Placeholder values {owner}, {repo} and {branch} in the endpoint are replaced
with values from the repository in the current directory, or the one given
with -R. When {owner} or {repo} is used, in the endpoint or a -F value, the
request goes to that repository's host, and --hostname or a full URL naming
another host is an error. Otherwise it goes to --hostname or the default host.

The default HTTP method is GET, or POST when parameters or --input are given.
Override it with --method.

Pass -f/--raw-field values in key=value format to add string parameters.
-F/--field converts its value by format:

- true, false, null and integers become JSON values of that type;
- {owner}, {repo} and {branch} are replaced as in the endpoint;
- a value starting with @ is read from the named file, or from standard
  input for @-.

Use key[subkey]=value for nested parameters, key[]=value to append to an
array, and a bare key[] for an empty array. With GET, parameters go to the
query string instead of a JSON body. With --input, the file is the body and
parameters go to the query string.

With --paginate, fj follows the Link header until the last page. Array pages
are merged into one array; other pages print one after another. --slurp wraps
every page in an outer array instead. Forgejo caps the page size at the
server's MAX_RESPONSE_ITEMS setting, 50 by default.

When the server answers with an error status, fj prints the response body
unfiltered and exits 1.`,
		Example: `  # List releases in the current repository
  $ fj api repos/{owner}/{repo}/releases

  # Post an issue comment
  $ fj api repos/{owner}/{repo}/issues/123/comments -f body='Hi from CLI'

  # Add parameters to a GET request
  $ fj api -X GET repos/search -f q=cli -F limit=5

  # Print the title of every open issue, across all pages
  $ fj api --paginate 'repos/{owner}/{repo}/issues?limit=50' --jq '.[].title'

  # Create a label from a JSON file
  $ fj api repos/{owner}/{repo}/labels --input label.json

  # Commit two files at once with an array of objects
  $ fj api repos/{owner}/{repo}/contents -f message='Add files' \
      -f 'files[][operation]=create' -f 'files[][path]=a.txt' -F 'files[][content]=@a.b64' \
      -f 'files[][operation]=create' -f 'files[][path]=b.txt' -F 'files[][content]=@b.b64'

  # Fetch the server's API reference
  $ fj api https://forgejo.example.com/swagger.v1.json`,
		Args: cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Endpoint = args[0]
			opts.MethodPassed = cmd.Flags().Changed("method")
			opts.Out = cmd.OutOrStdout()
			return apiRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Method, "method", "X", "GET", "The HTTP method for the request")
	cmd.Flags().StringArrayVarP(&opts.RawFields, "raw-field", "f", nil, "Add a string parameter in `key=value` format")
	cmd.Flags().StringArrayVarP(&opts.MagicFields, "field", "F", nil, "Add a typed parameter in `key=value` format (use \"@<path>\" or \"@-\" to read value from file or stdin)")
	cmd.Flags().StringArrayVarP(&opts.Headers, "header", "H", nil, "Add a HTTP request header in `key:value` format")
	cmd.Flags().StringVar(&opts.Input, "input", "", "The `file` to use as body for the HTTP request (use \"-\" to read from standard input)")
	cmd.Flags().BoolVarP(&opts.Include, "include", "i", false, "Include HTTP response status line and headers in the output")
	cmd.Flags().BoolVar(&opts.Silent, "silent", false, "Do not print the response body")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Log each request and response body to stderr, as DEBUG=3 does")
	cmd.Flags().BoolVar(&opts.Paginate, "paginate", false, "Make additional HTTP requests to fetch all pages of results")
	cmd.Flags().BoolVar(&opts.Slurp, "slurp", false, "Use with \"--paginate\" to return an array of all pages of either JSON arrays or objects")
	cmd.Flags().StringVarP(&opts.JQ, "jq", "q", "", "Query to select values from the response using jq syntax")
	cmd.Flags().StringVar(&f.HostOverride, "hostname", "", "The Forgejo hostname for the request")

	return cmd
}

func apiRun(opts *apiOptions) error {
	if opts.Slurp && !opts.Paginate {
		return cmdutil.FlagErrorf("`--paginate` required when passing `--slurp`")
	}
	if opts.Slurp && opts.JQ != "" {
		return cmdutil.FlagErrorf("`--slurp` is not supported with `--jq`")
	}
	// A second read of standard input gets an empty value, not an error.
	stdin := 0
	if opts.Input == "-" {
		stdin++
	}
	for _, s := range opts.MagicFields {
		if _, v, _ := strings.Cut(s, "="); v == "@-" {
			stdin++
		}
	}
	if stdin > 1 {
		return cmdutil.FlagErrorf("standard input can be read only once, by `--input -` or one `-F key=@-`")
	}

	var jq *gojq.Code
	if opts.JQ != "" {
		var err error
		if jq, err = output.ParseJQ(opts.JQ); err != nil {
			return err
		}
	}

	if opts.Verbose {
		debug.Raise(3)
	}

	headers := http.Header{}
	for _, h := range opts.Headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok || strings.TrimSpace(k) == "" {
			return cmdutil.FlagErrorf("header %q requires a value separated by a ':' sign", h)
		}
		headers.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	var ph placeholders
	if err := ph.resolve(opts); err != nil {
		return err
	}

	fields, err := parseFields(opts.RawFields, opts.MagicFields, ph.fill)
	if err != nil {
		return err
	}

	method := strings.ToUpper(opts.Method)
	if !opts.MethodPassed && (len(fields) > 0 || opts.Input != "") {
		method = http.MethodPost
	}
	if opts.Paginate && method != http.MethodGet {
		return cmdutil.FlagErrorf("the `--paginate` option is not supported for non-GET requests")
	}

	target := ph.fillPath(opts.Endpoint)
	var body []byte
	var contentType string
	switch {
	case opts.Input != "":
		data, err := cmdutil.ReadBodyFromFile(opts.Input)
		if err != nil {
			return err
		}
		body, contentType = []byte(data), "application/json"
		target = addQuery(target, fields)
	case method == http.MethodGet:
		target = addQuery(target, fields)
	case len(fields) > 0:
		payload, err := buildBody(fields)
		if err != nil {
			return err
		}
		if body, err = json.Marshal(payload); err != nil {
			return err
		}
		contentType = "application/json"
	}

	host, err := requestHost(opts, target, ph)
	if err != nil {
		return err
	}
	client, err := opts.Factory.APIClientForHost(host)
	if err != nil {
		return err
	}

	return send(opts, client, method, target, body, contentType, headers, jq)
}

// send runs the request, following Link headers under --paginate, and
// writes the output. Pages are buffered so array pages can be merged into
// one array before printing.
// ponytail: memory grows with the total result size; stream the merge if a
// listing ever outgrows RAM.
func send(opts *apiOptions, client *apiclient.Client, method, target string, body []byte, contentType string, headers http.Header, jq *gojq.Code) error {
	var pages [][]byte
	var pageType string
	for target != "" {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		resp, err := client.Raw(method, target, reader, contentType, headers)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		if opts.Include {
			writeHeaders(opts.Out, resp)
		}
		if resp.StatusCode > 299 {
			// Pages fetched before the error are printed, as --jq already
			// did when each arrived.
			writePages(opts, pages, pageType)
			if !opts.Silent {
				opts.Out.Write(data)
			}
			return httpError(resp.StatusCode, data)
		}

		target, body = "", nil
		if opts.Paginate {
			target = nextPage(resp.Header.Get("Link"))
		}

		switch {
		case opts.Silent:
		case jq != nil:
			// An empty body (204 No Content) has nothing to filter.
			if len(bytes.TrimSpace(data)) > 0 {
				if err := output.WriteJQ(opts.Out, jq, data); err != nil {
					return err
				}
			}
		case opts.Paginate:
			pages = append(pages, data)
			pageType = resp.Header.Get("Content-Type")
		default:
			writeBody(opts.Out, data, resp.Header.Get("Content-Type"))
		}
	}
	writePages(opts, pages, pageType)
	return nil
}

// writePages prints the pages buffered under --paginate.
func writePages(opts *apiOptions, pages [][]byte, pageType string) {
	switch {
	case len(pages) == 0:
	case opts.Slurp:
		writeBody(opts.Out, joinArray(pages), pageType)
	default:
		if merged, ok := mergeArrays(pages); ok {
			writeBody(opts.Out, merged, pageType)
			return
		}
		for _, p := range pages {
			writeBody(opts.Out, p, pageType)
		}
	}
}

// placeholders holds the values for {owner}, {repo} and {branch}. Each is
// looked up only when some input names it, so fj api works outside a git
// checkout for endpoints that need none.
type placeholders struct {
	owner, repo, branch string
	host                string // the repository's host, set when owner/repo were resolved
}

func (p *placeholders) resolve(opts *apiOptions) error {
	inputs := append([]string{opts.Endpoint}, opts.MagicFields...)
	uses := func(name string) bool {
		return slices.ContainsFunc(inputs, func(s string) bool { return strings.Contains(s, name) })
	}

	if uses("{owner}") || uses("{repo}") {
		repo, err := opts.Factory.BaseRepo()
		if err != nil {
			return err
		}
		p.owner, p.repo, p.host = repo.Owner, repo.Name, repo.Host
	}
	if uses("{branch}") {
		branch, err := git.CurrentBranch()
		if err != nil {
			return fmt.Errorf("resolving {branch}: %w", err)
		}
		p.branch = branch
	}
	return nil
}

// fill replaces placeholders in a field value, which is data, not a path.
func (p placeholders) fill(s string) string {
	return strings.NewReplacer("{owner}", p.owner, "{repo}", p.repo, "{branch}", p.branch).Replace(s)
}

// fillPath replaces placeholders in an endpoint, escaping each value for
// where it lands: a branch named feature/x must not add a path segment, an
// owner carrying "?" or "#" must not cut the path short, and a branch such
// as a&b=c+1 in the query string must stay one value.
func (p placeholders) fillPath(s string) string {
	replace := func(s string, escape func(string) string) string {
		return strings.NewReplacer(
			"{owner}", escape(p.owner),
			"{repo}", escape(p.repo),
			"{branch}", escape(p.branch),
		).Replace(s)
	}
	path, query, ok := strings.Cut(s, "?")
	s = replace(path, url.PathEscape)
	if ok {
		s += "?" + replace(query, url.QueryEscape)
	}
	return s
}

// requestHost picks the host the request goes to: the host of a full URL,
// then --hostname, then the host of the repository the placeholders came
// from, and last the shared resolver. When placeholders were filled, any
// other host is an error, so {owner}/{repo} from one server are never sent
// to another. A host fj has no token for is refused later, by TokenForHost.
func requestHost(opts *apiOptions, target string, ph placeholders) (string, error) {
	host := ph.host
	switch {
	case apiclient.IsAbsoluteURL(target):
		u, err := url.Parse(target)
		if err != nil {
			return "", err
		}
		host = u.Host
	case opts.Factory.HostOverride != "":
		host = opts.Factory.HostOverride
	case host == "":
		return opts.Factory.Host()
	}
	if ph.host != "" && !strings.EqualFold(host, ph.host) {
		return "", fmt.Errorf("{owner}/{repo} is %s/%s on %s, but the request goes to %s; use -R OWNER/REPO with --hostname %s",
			ph.owner, ph.repo, ph.host, host, host)
	}
	return host, nil
}

// field is one -f or -F parameter. value is a string, int, bool, nil, or
// emptyArray for a bare key[].
type field struct {
	key   string
	value any
}

type emptyArray struct{}

// parseFields parses -f values as strings and -F values with gh's type
// conversion. Raw fields come first, as in gh, since cobra does not keep the
// order across the two flags.
func parseFields(raw, magic []string, fill func(string) string) ([]field, error) {
	var fields []field
	for i, s := range append(slices.Clone(raw), magic...) {
		key, value, ok := strings.Cut(s, "=")
		if !ok {
			if strings.HasSuffix(key, "[]") {
				fields = append(fields, field{key, emptyArray{}})
				continue
			}
			return nil, cmdutil.FlagErrorf("field %q requires a value separated by an '=' sign", s)
		}
		if i < len(raw) {
			fields = append(fields, field{key, value})
			continue
		}
		v, err := magicValue(value, fill)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field{key, v})
	}
	return fields, nil
}

func magicValue(v string, fill func(string) string) (any, error) {
	if path, ok := strings.CutPrefix(v, "@"); ok {
		data, err := cmdutil.ReadBodyFromFile(path)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n, nil
	}
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	return fill(v), nil
}

// addQuery appends fields to target's query string, keys as typed.
func addQuery(target string, fields []field) string {
	q := url.Values{}
	for _, f := range fields {
		switch v := f.value.(type) {
		case emptyArray:
		case nil:
			q.Add(f.key, "")
		default:
			q.Add(f.key, fmt.Sprint(v))
		}
	}
	if len(q) == 0 {
		return target
	}
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	return target + sep + q.Encode()
}

// buildBody turns fields into a JSON object, expanding key[sub] into nested
// objects and key[] into arrays.
func buildBody(fields []field) (map[string]any, error) {
	body := map[string]any{}
	for _, f := range fields {
		path, err := parseKey(f.key)
		if err != nil {
			return nil, err
		}
		if err := setPath(body, path, f.value); err != nil {
			return nil, fmt.Errorf("field %q: %w", f.key, err)
		}
	}
	return body, nil
}

// parseKey splits a[b][] into ["a", "b", ""]. An empty segment means an
// array append.
func parseKey(key string) ([]string, error) {
	i := strings.IndexByte(key, '[')
	if i < 0 {
		return []string{key}, nil
	}
	if i == 0 {
		return nil, cmdutil.FlagErrorf("invalid key: %q", key)
	}
	segs := []string{key[:i]}
	for rest := key[i:]; rest != ""; {
		end := strings.IndexByte(rest, ']')
		if rest[0] != '[' || end < 0 {
			return nil, cmdutil.FlagErrorf("invalid key: %q", key)
		}
		segs = append(segs, rest[1:end])
		rest = rest[end+1:]
	}
	return segs, nil
}

func setPath(m map[string]any, segs []string, v any) error {
	key := segs[0]
	if len(segs) == 1 {
		m[key] = v
		return nil
	}

	if segs[1] != "" {
		child, ok := m[key].(map[string]any)
		if !ok {
			if _, exists := m[key]; exists {
				return fmt.Errorf("%q is already set to a non-object value", key)
			}
			child = map[string]any{}
			m[key] = child
		}
		return setPath(child, segs[1:], v)
	}

	arr, ok := m[key].([]any)
	if !ok {
		if _, exists := m[key]; exists {
			return fmt.Errorf("%q is already set to a non-array value", key)
		}
		arr = []any{}
	}
	rest := segs[2:]
	switch {
	case len(rest) == 0:
		if _, empty := v.(emptyArray); !empty {
			arr = append(arr, v)
		}
	case rest[0] == "":
		return fmt.Errorf("arrays of arrays are not supported")
	default:
		// key[][name]=v fills the last object in the array until a key would
		// be set twice, then starts a new object. That is how gh groups
		// consecutive fields into one array element.
		var obj map[string]any
		if n := len(arr); n > 0 {
			if last, ok := arr[n-1].(map[string]any); ok && !hasPath(last, rest) {
				obj = last
			}
		}
		if obj == nil {
			obj = map[string]any{}
			arr = append(arr, obj)
		}
		if err := setPath(obj, rest, v); err != nil {
			return err
		}
	}
	m[key] = arr
	return nil
}

// hasPath reports whether setting segs in m would overwrite a value or nest
// under one that is not an object. An array append never does.
func hasPath(m map[string]any, segs []string) bool {
	v, ok := m[segs[0]]
	if !ok {
		return false
	}
	if len(segs) == 1 {
		return true
	}
	if segs[1] == "" {
		return false
	}
	child, ok := v.(map[string]any)
	return !ok || hasPath(child, segs[1:])
}

var linkNext = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// nextPage returns the target for the page after this one, or "" on the
// last page. Forgejo builds Link URLs from its ROOT_URL setting, which need
// not match the host fj reached (a reverse proxy, a mapped port), so only
// the part after /api/v1/ is kept and the next page goes where the first
// one went.
func nextPage(link string) string {
	m := linkNext.FindStringSubmatch(link)
	if m == nil {
		return ""
	}
	if _, rest, ok := strings.Cut(m[1], "/api/v1/"); ok {
		return rest
	}
	return m[1]
}

// mergeArrays joins pages that are all JSON arrays into one array, keeping
// each element's bytes as the server sent them. It reports false when any
// page is not an array.
func mergeArrays(pages [][]byte) ([]byte, bool) {
	var items []json.RawMessage
	for _, p := range pages {
		var page []json.RawMessage
		if err := json.Unmarshal(p, &page); err != nil || page == nil {
			return nil, false
		}
		items = append(items, page...)
	}
	return joinArray(items), true
}

// joinArray writes elements as a JSON array: [a,b,c].
func joinArray[T ~[]byte](elems []T) []byte {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, e := range elems {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(bytes.TrimSpace(e))
	}
	b.WriteString("]\n")
	return b.Bytes()
}

// writeBody prints a response body: indented when JSON goes to a terminal,
// byte for byte otherwise, so piped output and non-JSON bodies (a raw file,
// a diff, an archive) arrive unchanged.
func writeBody(w io.Writer, data []byte, contentType string) {
	if isTerminal(w) && isJSON(contentType) {
		var buf bytes.Buffer
		if json.Indent(&buf, data, "", "  ") == nil {
			w.Write(append(bytes.TrimRight(buf.Bytes(), "\n"), '\n'))
			return
		}
	}
	w.Write(data)
}

func writeHeaders(w io.Writer, resp *http.Response) {
	fmt.Fprintf(w, "%s %s\n", resp.Proto, resp.Status)
	for _, k := range slices.Sorted(maps.Keys(resp.Header)) {
		for _, v := range resp.Header[k] {
			fmt.Fprintf(w, "%s: %s\n", k, v)
		}
	}
	fmt.Fprintln(w)
}

// httpError reports a non-2xx status the way gh does, "Not Found (HTTP 404)",
// using the server's message when the body carries one.
func httpError(status int, data []byte) error {
	msg := http.StatusText(status)
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Message != "" {
		msg = payload.Message
	}
	return fmt.Errorf("%s (HTTP %d)", msg, status)
}

func isJSON(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mt == "application/json" || strings.HasSuffix(mt, "+json"))
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
