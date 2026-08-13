package pr

import (
	"strings"
	"testing"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestSectionLines(t *testing.T) {
	entry := func(number int64) prStatusEntry {
		return prStatusEntry{Number: number, Title: "Fix it", Branch: "feature-1"}
	}

	t.Run("empty section has no lines so the empty state prints", func(t *testing.T) {
		if got := sectionLines(nil); len(got) != 0 {
			t.Errorf("sectionLines(nil) = %q, want no lines", got)
		}
	})

	t.Run("row names the number, title and head branch", func(t *testing.T) {
		got := sectionLines([]prStatusEntry{entry(6)})
		if len(got) != 1 {
			t.Fatalf("sectionLines = %q, want 1 line", got)
		}
		for _, want := range []string{"#6", "Fix it", "[feature-1]"} {
			if !strings.Contains(got[0], want) {
				t.Errorf("line = %q, want it to contain %q", got[0], want)
			}
		}
	})

	t.Run("checks line follows its row", func(t *testing.T) {
		e := entry(6)
		e.Checks = string(forgejo.StatusSuccess)
		got := sectionLines([]prStatusEntry{e})
		want := []string{"#6", "✓ Checks passing"}
		if len(got) != len(want) {
			t.Fatalf("sectionLines = %q, want %d lines", got, len(want))
		}
		for i := range want {
			if !strings.Contains(got[i], want[i]) {
				t.Errorf("line %d = %q, want it to contain %q", i, got[i], want[i])
			}
		}
	})

	t.Run("tail counts pull requests beyond the cap", func(t *testing.T) {
		entries := make([]prStatusEntry, statusLimit+3)
		for i := range entries {
			entries[i] = entry(int64(i + 1))
		}
		got := sectionLines(entries)
		if len(got) != statusLimit+1 {
			t.Fatalf("sectionLines returned %d lines, want %d", len(got), statusLimit+1)
		}
		if last := got[len(got)-1]; !strings.Contains(last, "And 3 more") {
			t.Errorf("tail = %q, want it to contain %q", last, "And 3 more")
		}
	})
}

func TestChecksLine(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		// A commit nothing ever reported on must not claim success.
		{"", ""},
		{string(forgejo.StatusSuccess), "✓ Checks passing"},
		{string(forgejo.StatusPending), "- Checks pending"},
		{string(forgejo.StatusFailure), "× Checks failing"},
		{string(forgejo.StatusError), "× Checks failing"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := checksLine(tt.state); got != tt.want {
				t.Errorf("checksLine(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}
