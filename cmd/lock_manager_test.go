package cmd

import "testing"

func TestParseTmuxOwnerID(t *testing.T) {
	t.Run("session and window", func(t *testing.T) {
		sessionID, windowID, ok := parseTmuxOwnerID("tmux:$1:@2")
		if !ok {
			t.Fatalf("expected tmux owner to parse")
		}
		if sessionID != "$1" {
			t.Fatalf("expected session $1, got %q", sessionID)
		}
		if windowID != "@2" {
			t.Fatalf("expected window @2, got %q", windowID)
		}
	})

	t.Run("session only", func(t *testing.T) {
		sessionID, windowID, ok := parseTmuxOwnerID("tmux:$9")
		if !ok {
			t.Fatalf("expected tmux owner to parse")
		}
		if sessionID != "$9" {
			t.Fatalf("expected session $9, got %q", sessionID)
		}
		if windowID != "" {
			t.Fatalf("expected empty window, got %q", windowID)
		}
	})

	t.Run("invalid owner", func(t *testing.T) {
		if _, _, ok := parseTmuxOwnerID("term-session:abc"); ok {
			t.Fatalf("expected non-tmux owner to fail parsing")
		}
		if _, _, ok := parseTmuxOwnerID("tmux:"); ok {
			t.Fatalf("expected empty tmux owner to fail parsing")
		}
	})
}

func TestLockOwnerStillActive_UnknownOwnerWithoutPID(t *testing.T) {
	if lockOwnerStillActive("", 0) {
		t.Fatalf("expected empty owner without pid to be inactive")
	}
}
