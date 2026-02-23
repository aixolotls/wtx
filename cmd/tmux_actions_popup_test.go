package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestTmuxActionsCommandWithPathAndAction(t *testing.T) {
	got := tmuxActionsCommandWithPathAndAction("/usr/local/bin/wtx", "/tmp/repo path", tmuxActionRename)
	if want := "tmux-actions"; !strings.Contains(got, want) {
		t.Fatalf("expected %q in %q", want, got)
	}
	if want := "'/tmp/repo path'"; !strings.Contains(got, want) {
		t.Fatalf("expected quoted path %q in %q", want, got)
	}
	if want := "rename_branch"; !strings.Contains(got, want) {
		t.Fatalf("expected action %q in %q", want, got)
	}
}

func TestRenameCurrentBranch_Succeeds(t *testing.T) {
	repo := initRenameTestRepo(t)
	runGitInRepo(t, repo, "checkout", "-b", "before-rename")

	if err := renameCurrentBranch(repo, "after-rename"); err != nil {
		t.Fatalf("renameCurrentBranch failed: %v", err)
	}

	head := strings.TrimSpace(runGitOutput(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))
	if head != "after-rename" {
		t.Fatalf("expected HEAD to be after-rename, got %q", head)
	}
}

func TestRenameCurrentBranch_TimesOut(t *testing.T) {
	repo := initRenameTestRepo(t)
	runGitInRepo(t, repo, "checkout", "-b", "before-rename")

	prev := renameCurrentBranchTimeout
	renameCurrentBranchTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		renameCurrentBranchTimeout = prev
	})

	fakeBinDir := t.TempDir()
	gitName := "git"
	script := "#!/bin/sh\nsleep 1\n"
	if runtime.GOOS == "windows" {
		gitName = "git.bat"
		script = "@echo off\r\nping -n 2 127.0.0.1 >NUL\r\n"
	}
	fakeGitPath := filepath.Join(fakeBinDir, gitName)
	if err := os.WriteFile(fakeGitPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	start := time.Now()
	err := renameCurrentBranch(repo, "after-rename")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout message, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("expected fail-fast timeout; took %s", elapsed)
	}
}

func initRenameTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitInRepo(t, dir, "init")
	runGitInRepo(t, dir, "config", "user.name", "Test User")
	runGitInRepo(t, dir, "config", "user.email", "test@example.com")

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitInRepo(t, dir, "add", "README.md")
	runGitInRepo(t, dir, "commit", "-m", "seed")
	return dir
}

func runGitInRepo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, strings.TrimSpace(string(out)))
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

func TestResolveTmuxActionsBasePathFromCandidates(t *testing.T) {
	t.Run("uses first non-empty candidate", func(t *testing.T) {
		got := resolveTmuxActionsBasePathFromCandidates(
			"",
			"/tmp/option",
			"/tmp/session-option",
			"/tmp/session-env",
			"/tmp/cwd",
		)
		if got != "/tmp/option" {
			t.Fatalf("expected /tmp/option, got %q", got)
		}
	})

	t.Run("falls back through session metadata then cwd", func(t *testing.T) {
		got := resolveTmuxActionsBasePathFromCandidates(
			"",
			"",
			"",
			"/tmp/session-env",
			"/tmp/cwd",
		)
		if got != "/tmp/session-env" {
			t.Fatalf("expected /tmp/session-env, got %q", got)
		}

		got = resolveTmuxActionsBasePathFromCandidates("", "", "", "", "/tmp/cwd")
		if got != "/tmp/cwd" {
			t.Fatalf("expected /tmp/cwd, got %q", got)
		}
	})
}
