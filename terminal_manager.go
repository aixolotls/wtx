package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func setITermWTXTab() {
	setITermTab("wtx")
}

func setITermWTXBranchTab(branch string) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		setITermWTXTab()
		return
	}
	setITermTab("wtx - " + branch)
}

func setITermTab(title string) {
	inTmux := strings.TrimSpace(os.Getenv("TMUX")) != ""
	if !inTmux && strings.TrimSpace(os.Getenv("TERM_PROGRAM")) != "iTerm.app" {
		return
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "wtx"
	}
	// OSC 0/1/2 set title; use both 1337 and legacy color controls for iTerm.
	writeTerminalEscape("\x1b]0;" + title + "\x07")
	writeTerminalEscape("\x1b]1;" + title + "\x07")
	writeTerminalEscape("\x1b]2;" + title + "\x07")
	writeTerminalEscape("\x1b]1337;SetTabColor=rgb:7d/56/f4\x07")
	writeTerminalEscape("\x1b]6;1;bg;red;brightness;125\x07")
	writeTerminalEscape("\x1b]6;1;bg;green;brightness;86\x07")
	writeTerminalEscape("\x1b]6;1;bg;blue;brightness;244\x07")
	if inTmux {
		if sessionID, err := currentSessionID(); err == nil && strings.TrimSpace(sessionID) != "" {
			_ = exec.Command("tmux", "set-option", "-t", sessionID, "set-titles", "on").Run()
			_ = exec.Command("tmux", "set-option", "-t", sessionID, "set-titles-string", title).Run()
		}
		_ = exec.Command("tmux", "set-window-option", "-q", "automatic-rename", "off").Run()
		_ = exec.Command("tmux", "rename-window", title).Run()
	}
}

func resetITermTabColor() {
	inTmux := strings.TrimSpace(os.Getenv("TMUX")) != ""
	if !inTmux && strings.TrimSpace(os.Getenv("TERM_PROGRAM")) != "iTerm.app" {
		return
	}
	// Clear iTerm custom tab color and let defaults apply.
	writeTerminalEscape("\x1b]1337;SetTabColor=\x07")
}

func writeTerminalEscape(seq string) {
	if strings.TrimSpace(seq) == "" {
		return
	}
	// When inside tmux, wrap OSC sequences so iTerm receives them.
	if strings.TrimSpace(os.Getenv("TMUX")) != "" {
		escaped := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
		fmt.Fprint(os.Stdout, "\x1bPtmux;", escaped, "\x1b\\")
		return
	}
	fmt.Fprint(os.Stdout, seq)
}
