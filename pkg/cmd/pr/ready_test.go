package pr

import "testing"

func TestStripWIP(t *testing.T) {
	tests := []struct{ title, want string }{
		{"WIP: Fix crash", "Fix crash"},
		{"wip:Fix crash", "Fix crash"},
		{"[WIP] Fix crash", "Fix crash"},
		{"[wip]  WIP: Fix crash", "Fix crash"},
		{"Fix WIP: crash", "Fix WIP: crash"},
		{"WIPE the cache", "WIPE the cache"},
		{"WIP", "WIP"},
		{"[WIP]", ""},
	}
	for _, tt := range tests {
		if got := stripWIP(tt.title); got != tt.want {
			t.Errorf("stripWIP(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}
