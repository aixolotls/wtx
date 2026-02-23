package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestActionMatchesQuery_Substring(t *testing.T) {
	item := tmuxActionItem{
		Label:    "Open shell (split down)",
		Action:   tmuxActionShellSplit,
		Keywords: "shell split pane",
	}
	if !actionMatchesQuery(item, "split") {
		t.Fatalf("expected substring query to match")
	}
}

func TestActionMatchesQuery_TokenPrefix(t *testing.T) {
	item := tmuxActionItem{
		Label:    "Open IDE",
		Action:   tmuxActionIDE,
		Keywords: "editor code",
	}
	if !actionMatchesQuery(item, "edi") {
		t.Fatalf("expected token prefix query to match")
	}
}

func TestActionMatchesQuery_DoesNotOvermatchShortQuery(t *testing.T) {
	item := tmuxActionItem{
		Label:    "Open shell (split down)",
		Action:   tmuxActionShellSplit,
		Keywords: "shell split pane ctrl+s s",
	}
	if actionMatchesQuery(item, "pr") {
		t.Fatalf("expected short query pr not to match shell action")
	}
}

func TestTmuxActionsModel_RebuildFiltered(t *testing.T) {
	m := newTmuxActionsModel("/tmp", true, false, false)
	m.query = "pull"
	m.rebuildFiltered()
	item, ok := m.selectedItem()
	if !ok {
		t.Fatalf("expected a selected item after filtering")
	}
	if item.Action != tmuxActionPR {
		t.Fatalf("expected PR action, got %q", item.Action)
	}
}

func TestParseTmuxAction_BackToWTX(t *testing.T) {
	got := parseTmuxAction("back_to_wtx")
	if got != tmuxActionBack {
		t.Fatalf("expected back_to_wtx action, got %q", got)
	}
}

func TestParseTmuxAction_RenameBranch(t *testing.T) {
	got := parseTmuxAction("rename_branch")
	if got != tmuxActionRename {
		t.Fatalf("expected rename_branch action, got %q", got)
	}
}

func TestParseTmuxAction_ShellWindow(t *testing.T) {
	got := parseTmuxAction("shell_window")
	if got != tmuxActionShellWindow {
		t.Fatalf("expected shell_window action, got %q", got)
	}
}

func TestTmuxActionsModel_CtrlWSelectsBack(t *testing.T) {
	m := newTmuxActionsModel("/tmp", true, false, false)
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	updated := updatedModel.(tmuxActionsModel)
	if updated.chosen != tmuxActionBack {
		t.Fatalf("expected ctrl+w to choose back action, got %q", updated.chosen)
	}
}

func TestTmuxActionsModel_CtrlRSelectsRename(t *testing.T) {
	m := newTmuxActionsModel("/tmp", true, false, false)
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	updated := updatedModel.(tmuxActionsModel)
	if updated.chosen != tmuxActionRename {
		t.Fatalf("expected ctrl+r to choose rename action, got %q", updated.chosen)
	}
}

func TestTmuxActionsModel_ShowsShellWindowActionDisabledWhenUnavailable(t *testing.T) {
	m := newTmuxActionsModel("/tmp", true, false, false)
	found := false
	for _, item := range m.items {
		if item.Action != tmuxActionShellWindow {
			continue
		}
		found = true
		if !item.Disabled {
			t.Fatalf("expected shell window action to be disabled when unavailable")
		}
	}
	if !found {
		t.Fatalf("expected shell window action to exist")
	}
}

func TestTmuxActionsModel_ShowsShellTabActionDisabledWhenUnavailable(t *testing.T) {
	m := newTmuxActionsModel("/tmp", true, false, false)
	found := false
	for _, item := range m.items {
		if item.Action != tmuxActionShellTab {
			continue
		}
		found = true
		if !item.Disabled {
			t.Fatalf("expected shell tab action to be disabled when unavailable")
		}
	}
	if !found {
		t.Fatalf("expected shell tab action to exist")
	}
}

func TestTmuxActionsModel_ViewShowsShortcutHints(t *testing.T) {
	m := newTmuxActionsModel("/tmp", true, false, false)
	view := m.View()
	if !strings.Contains(view, "ctrl+w back") {
		t.Fatalf("expected ctrl+w hint in view, got %q", view)
	}
	if !strings.Contains(view, "ctrl+r rename") {
		t.Fatalf("expected ctrl+r hint in view, got %q", view)
	}
}

func TestTmuxActionsCommandWithAction_InjectsSourcePane(t *testing.T) {
	got := tmuxActionsCommandWithAction("/usr/local/bin/wtx", tmuxActionBack)
	if strings.Contains(got, "--source-pane") {
		t.Fatalf("did not expect source-pane flag in %q", got)
	}
	if want := "back_to_wtx"; !strings.Contains(got, want) {
		t.Fatalf("expected back action token %q in %q", want, got)
	}
}
