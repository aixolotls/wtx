package cmd

import (
	"strings"
	"testing"
)

func TestRenderOpenScreenTmuxHintShownBelowUpdateHint(t *testing.T) {
	t.Setenv("WTX_DISABLE_TMUX", "1")
	t.Setenv("TMUX", "")

	updateHint := "Update available: v1.2.3"
	view := renderOpenScreen(model{
		openStage:   openStageMain,
		updateHint:  updateHint,
		openLoading: true,
	})

	tmuxHint := "tmux not detected; status line is disabled."
	updateIdx := strings.Index(view, updateHint)
	tmuxIdx := strings.Index(view, tmuxHint)
	if updateIdx == -1 {
		t.Fatalf("expected update hint to be present, got %q", view)
	}
	if tmuxIdx == -1 {
		t.Fatalf("expected tmux hint to be present, got %q", view)
	}
	if tmuxIdx <= updateIdx {
		t.Fatalf("expected tmux hint below update hint, got %q", view)
	}
}

func TestRenderOpenScreenAlignsPRColumnByDetectedMaxBranchLength(t *testing.T) {
	t.Setenv("WTX_DISABLE_TMUX", "1")
	t.Setenv("TMUX", "")

	view := renderOpenScreen(model{
		openStage: openStageMain,
		openBranches: []openBranchOption{
			{Name: "short", HasPR: true, PRNumber: 1},
			{Name: "much-longer-branch-name", HasPR: true, PRNumber: 2},
		},
		openLockedBranches: []openBranchOption{
			{Name: "mid", HasPR: true, PRNumber: 3},
		},
	})

	shortLine := findRenderedLine(view, "short")
	longLine := findRenderedLine(view, "much-longer-branch-name")
	lockedLine := findRenderedLine(view, "mid")
	if shortLine == "" || longLine == "" || lockedLine == "" {
		t.Fatalf("expected rendered lines for all branches, got %q", view)
	}

	shortPR := strings.Index(shortLine, "#1")
	longPR := strings.Index(longLine, "#2")
	lockedPR := strings.Index(lockedLine, "#3")
	if shortPR == -1 || longPR == -1 || lockedPR == -1 {
		t.Fatalf("expected PR markers on all rows, got %q", view)
	}
	if shortPR != longPR || shortPR != lockedPR {
		t.Fatalf("expected aligned PR columns, got short=%d long=%d locked=%d\n%s", shortPR, longPR, lockedPR, view)
	}
}

func findRenderedLine(view string, needle string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
