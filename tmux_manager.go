package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func ensureFreshTmuxSession(args []string) (bool, error) {
	if strings.TrimSpace(os.Getenv("TMUX")) != "" {
		return false, nil
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return false, nil
	}

	bin, err := resolveSelfBinary(args)
	if err != nil {
		return false, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}

	setITermWTXTab()

	session := fmt.Sprintf("wtx-%d", time.Now().UnixNano())
	tmuxArgs := []string{"new-session", "-s", session, "-c", cwd, bin}
	if len(args) > 1 {
		tmuxArgs = append(tmuxArgs, args[1:]...)
	}
	cmd := exec.Command("tmux", tmuxArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return true, nil
}

func resolveSelfBinary(args []string) (string, error) {
	arg0 := strings.TrimSpace(args[0])
	if arg0 == "" {
		return "", fmt.Errorf("unable to resolve executable path")
	}
	if filepath.IsAbs(arg0) {
		return arg0, nil
	}
	if strings.Contains(arg0, string(os.PathSeparator)) {
		abs, err := filepath.Abs(arg0)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	path, err := exec.LookPath(arg0)
	if err != nil {
		return "", err
	}
	return path, nil
}

func setStartupStatusBanner() {
	if strings.TrimSpace(os.Getenv("TMUX")) == "" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	setStatusBanner(renderBanner("", cwd))
}

func splitAgentPane(worktreePath string, agentCmd string) (string, error) {
	cmd := exec.Command("tmux", "split-window", "-v", "-p", "70", "-d", "-c", worktreePath, "-P", "-F", "#{pane_id}", "/bin/sh", "-lc", agentCmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func tmuxAvailable() bool {
	if strings.TrimSpace(os.Getenv("TMUX")) == "" {
		return false
	}
	_, err := exec.LookPath("tmux")
	return err == nil
}

func currentPaneID() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_id}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func currentSessionID() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_id}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func renderBanner(branch string, path string) string {
	label := "WTX"
	if branch != "" {
		label = label + "  " + branch
	}
	if path != "" {
		label = label + "  " + path
	}
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFF7DB")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)
	return style.Render(label)
}

func setStatusBanner(banner string) {
	if strings.TrimSpace(banner) == "" {
		return
	}
	banner = stripANSI(banner)
	sessionID, err := currentSessionID()
	if err != nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	_ = exec.Command("tmux", "set-option", "-t", sessionID, "status", "1").Run()
	_ = exec.Command("tmux", "set-option", "-t", sessionID, "status-position", "bottom").Run()
	_ = exec.Command("tmux", "set-option", "-t", sessionID, "status-justify", "left").Run()
	_ = exec.Command("tmux", "set-option", "-t", sessionID, "status-style", "fg=#FFF7DB,bg=#7D56F4").Run()
	_ = exec.Command("tmux", "set-option", "-t", sessionID, "status-left-length", "200").Run()
	_ = exec.Command("tmux", "set-option", "-t", sessionID, "status-right", "").Run()
	_ = exec.Command("tmux", "set-option", "-t", sessionID, "status-left", " "+banner+" ").Run()
}

func clearScreen() {
	_ = exec.Command("tmux", "clear-history").Run()
	fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
}

func stripANSI(value string) string {
	out := make([]rune, 0, len(value))
	inEscape := false
	for _, r := range value {
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
