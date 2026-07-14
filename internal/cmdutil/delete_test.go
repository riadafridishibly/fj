package cmdutil

import "testing"

// TestResolveDeleteFlags covers the safety crux shared by every delete command:
// exactly one of --yes / --dry-run must be given. The error cases must surface
// as flag errors so that main exits with code 2 (see IsFlagError), and the
// happy paths must report whether the caller should perform the deletion.
func TestResolveDeleteFlags(t *testing.T) {
	tests := []struct {
		name        string
		yes         bool
		dryRun      bool
		wantPerform bool
		wantErr     bool
	}{
		{
			name:        "yes performs the deletion",
			yes:         true,
			dryRun:      false,
			wantPerform: true,
			wantErr:     false,
		},
		{
			name:        "dry-run does not perform the deletion",
			yes:         false,
			dryRun:      true,
			wantPerform: false,
			wantErr:     false,
		},
		{
			name:        "neither flag refuses to delete",
			yes:         false,
			dryRun:      false,
			wantPerform: false,
			wantErr:     true,
		},
		{
			name:        "both flags are mutually exclusive",
			yes:         true,
			dryRun:      true,
			wantPerform: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perform, err := ResolveDeleteFlags(tt.yes, tt.dryRun)
			if perform != tt.wantPerform {
				t.Errorf("perform = %v, want %v", perform, tt.wantPerform)
			}
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !IsFlagError(err) {
				t.Errorf("expected a flag error (exit code 2), got %T: %v", err, err)
			}
		})
	}
}
