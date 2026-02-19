package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "wtx error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 1 {
		switch args[1] {
		case "config":
			p := tea.NewProgram(newConfigModel(), tea.WithMouseCellMotion())
			return p.Start()
		case "tmux-status":
			return runTmuxStatus(args[2:])
		case "tmux-title":
			return runTmuxTitle(args[2:])
		case "tmux-agent-start":
			return runTmuxAgentStart(args[2:])
		case "tmux-agent-exit":
			return runTmuxAgentExit(args[2:])
		case "shell":
			return runShell()
		case "ide":
			return runIDE(args[2:])
		default:
			return fmt.Errorf("unknown command: %s", args[1])
		}
	}

	handled, err := ensureFreshTmuxSession(args)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	exists, err := ConfigExists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("wtx not configured. run: wtx config")
	}

	setITermWTXTab()
	setStartupStatusBanner()

	shouldResetTabColor := true
	defer func() {
		if shouldResetTabColor {
			resetITermTabColor()
		}
	}()

	p := tea.NewProgram(newModel(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := finalModel.(model); ok {
		path, branch, openShell, lock := m.PendingWorktree()
		if strings.TrimSpace(path) != "" {
			shouldResetTabColor = false
			runner := NewRunner(NewLockManager())
			if openShell {
				if _, err := runner.RunShellInWorktree(path, branch, lock); err != nil {
					if lock != nil {
						lock.Release()
					}
					return err
				}
			} else {
				if _, err := runner.RunInWorktree(path, branch, lock); err != nil {
					if lock != nil {
						lock.Release()
					}
					return err
				}
			}
		}
	}
	return nil
}
