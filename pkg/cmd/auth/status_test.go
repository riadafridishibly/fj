package auth

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/riadafridishibly/fj/internal/cmdutil"
)

// runStatus runs statusRun and returns what it printed.
func runStatus(t *testing.T, opts *statusOptions) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = statusRun(opts)
	w.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	return string(out)
}

// TestStatusHostOrder: hosts are listed by name on every run, not in map
// order. The hosts refuse connections, so each one fails fast.
func TestStatusHostOrder(t *testing.T) {
	want := []string{"127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "127.0.0.1:4", "127.0.0.1:5"}
	f, _ := factoryWithHosts(want[3], want[0], want[4], want[2], want[1])

	for range 10 {
		var got []string
		for _, line := range strings.Split(runStatus(t, &statusOptions{Factory: f}), "\n") {
			if line != "" && !strings.HasPrefix(line, " ") {
				got = append(got, line)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("hosts = %v, want %v", got, want)
		}
	}
}

// TestStatusJSON: a failed token check is gh's error state, with the reason.
func TestStatusJSON(t *testing.T) {
	f, _ := factoryWithHosts("127.0.0.1:1")
	out := runStatus(t, &statusOptions{Factory: f, JSONOutput: cmdutil.JSONFlags{Fields: []string{"hosts"}}})

	var got struct{ Hosts map[string][]map[string]any }
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parsing %s: %v", out, err)
	}
	accounts := got.Hosts["127.0.0.1:1"]
	if len(accounts) != 1 {
		t.Fatalf("hosts = %s, want one account on 127.0.0.1:1", out)
	}
	a := accounts[0]
	if e, _ := a["error"].(string); a["active"] != true || a["gitProtocol"] != "https" ||
		a["host"] != "127.0.0.1:1" || a["login"] != "me" || a["state"] != "error" || e == "" {
		t.Errorf("account = %v", a)
	}
}
