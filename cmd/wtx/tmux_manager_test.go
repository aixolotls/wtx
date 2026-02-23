package main

import "testing"

func TestParseBoolArg(t *testing.T) {
	if !parseBoolArg([]string{"--worktree", "/tmp/wt.1", "--force-unlock"}, "--force-unlock") {
		t.Fatalf("expected --force-unlock to be detected")
	}
	if parseBoolArg([]string{"--worktree", "/tmp/wt.1"}, "--force-unlock") {
		t.Fatalf("did not expect --force-unlock when flag is absent")
	}
}

func TestTmuxMouseEnabledForCurrentTerminal(t *testing.T) {
	t.Run("enabled via TERM_PROGRAM map", func(t *testing.T) {
		t.Setenv("TERM_PROGRAM", "ghostty")
		t.Setenv("TERM", "xterm-256color")
		if !tmuxMouseEnabledForCurrentTerminal() {
			t.Fatalf("expected ghostty TERM_PROGRAM to enable tmux mouse mode")
		}
	})

	t.Run("enabled via TERM fallback", func(t *testing.T) {
		t.Setenv("TERM_PROGRAM", "")
		t.Setenv("TERM", "xterm-ghostty")
		if !tmuxMouseEnabledForCurrentTerminal() {
			t.Fatalf("expected TERM fallback to enable tmux mouse mode for ghostty")
		}
	})

	t.Run("disabled by default for other terminals", func(t *testing.T) {
		t.Setenv("TERM_PROGRAM", "iTerm.app")
		t.Setenv("TERM", "xterm-256color")
		if tmuxMouseEnabledForCurrentTerminal() {
			t.Fatalf("expected non-mapped terminals to keep tmux mouse mode disabled")
		}
	})
}
