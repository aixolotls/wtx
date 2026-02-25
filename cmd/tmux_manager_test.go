package cmd

import "testing"

func TestParseBoolArg(t *testing.T) {
	if !parseBoolArg([]string{"--worktree", "/tmp/wt.1", "--force-unlock"}, "--force-unlock") {
		t.Fatalf("expected --force-unlock to be detected")
	}
	if parseBoolArg([]string{"--worktree", "/tmp/wt.1"}, "--force-unlock") {
		t.Fatalf("did not expect --force-unlock when flag is absent")
	}
}

func TestShouldStartIsolatedTmuxSession(t *testing.T) {
	tests := []struct {
		name          string
		current       string
		sessionParent string
		want          bool
	}{
		{
			name:          "same terminal does not isolate",
			current:       "Ghostty",
			sessionParent: "ghostty",
			want:          false,
		},
		{
			name:          "different terminal isolates",
			current:       "Apple_Terminal",
			sessionParent: "Ghostty",
			want:          true,
		},
		{
			name:          "missing session metadata does not isolate",
			current:       "Apple_Terminal",
			sessionParent: "",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldStartIsolatedTmuxSession(tt.current, tt.sessionParent); got != tt.want {
				t.Fatalf("shouldStartIsolatedTmuxSession(%q, %q)=%v, want %v", tt.current, tt.sessionParent, got, tt.want)
			}
		})
	}
}
