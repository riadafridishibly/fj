package auth

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"testing"
)

// TestStatusHostOrder: hosts are listed by name on every run, not in map
// order. The hosts refuse connections, so each one fails fast.
func TestStatusHostOrder(t *testing.T) {
	want := []string{"127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "127.0.0.1:4", "127.0.0.1:5"}
	f, _ := factoryWithHosts(want[3], want[0], want[4], want[2], want[1])

	for range 10 {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout := os.Stdout
		os.Stdout = w
		err = statusRun(&statusOptions{Factory: f, JSONOutput: true})
		w.Close()
		os.Stdout = stdout
		if err != nil {
			t.Fatal(err)
		}
		out, _ := io.ReadAll(r)

		var statuses []struct{ Hostname string }
		if err := json.Unmarshal(out, &statuses); err != nil {
			t.Fatalf("parsing %s: %v", out, err)
		}
		var got []string
		for _, s := range statuses {
			got = append(got, s.Hostname)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("hosts = %v, want %v", got, want)
		}
	}
}
