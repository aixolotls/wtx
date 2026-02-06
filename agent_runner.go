package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Runner struct {
	lockMgr *LockManager
}

func NewRunner() *Runner {
	return &Runner{lockMgr: NewLockManager()}
}

type AgentRunResult struct {
	Started bool
	Warning string
}

func (r *Runner) RunInWorktree(worktreePath string, branch string, lock *WorktreeLock) (AgentRunResult, error) {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return AgentRunResult{}, errors.New("worktree path required")
	}
	branch = strings.TrimSpace(branch)

	cfg, err := LoadConfig()
	if err != nil {
		return AgentRunResult{}, err
	}

	agentCmd := strings.TrimSpace(cfg.AgentCommand)
	if agentCmd == "" {
		agentCmd = defaultAgentCommand
	}

	return r.runInWorktree(worktreePath, branch, lock, false, agentCmd)
}

func (r *Runner) RunShellInWorktree(worktreePath string, branch string, lock *WorktreeLock) (AgentRunResult, error) {
	return r.runInWorktree(worktreePath, branch, lock, true, "")
}

func (r *Runner) runInWorktree(worktreePath string, branch string, lock *WorktreeLock, openShell bool, agentCmd string) (AgentRunResult, error) {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return AgentRunResult{}, errors.New("worktree path required")
	}
	branch = strings.TrimSpace(branch)

	if tmuxAvailable() {
		return r.runInTmux(worktreePath, branch, lock, openShell, agentCmd)
	}
	return r.runWithoutTmux(worktreePath, branch, lock, openShell, agentCmd)
}

func (r *Runner) runInTmux(worktreePath string, branch string, lock *WorktreeLock, openShell bool, agentCmd string) (AgentRunResult, error) {
	paneID, _ := currentPaneID()
	var (
		newPaneID string
		err       error
	)
	if openShell {
		newPaneID, err = splitShellPane(worktreePath)
	} else {
		newPaneID, err = splitAgentPane(worktreePath, agentCmd)
	}
	if err != nil {
		return AgentRunResult{}, err
	}
	if err := r.lockWorktreeForPane(worktreePath, newPaneID, lock); err != nil {
		return AgentRunResult{}, err
	}
	clearScreen()
	setDynamicWorktreeStatus(worktreePath)
	setITermWTXBranchTab(branch)
	if paneID != "" {
		_ = exec.Command("tmux", "resize-pane", "-t", paneID, "-y", "1").Run()
	}
	if newPaneID != "" {
		_ = exec.Command("tmux", "select-pane", "-t", newPaneID).Run()
	}
	return AgentRunResult{Started: true}, nil
}

func (r *Runner) runWithoutTmux(worktreePath string, branch string, lock *WorktreeLock, openShell bool, agentCmd string) (AgentRunResult, error) {
	cmd := shellCommand(worktreePath, openShell, agentCmd)
	if err := cmd.Start(); err != nil {
		return AgentRunResult{}, err
	}
	boundLock, err := r.lockWorktreeForPID(worktreePath, cmd.Process.Pid, lock)
	if err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return AgentRunResult{}, err
	}
	if boundLock != nil {
		defer boundLock.Release()
	}

	clearScreen()
	setDynamicWorktreeStatus(worktreePath)
	setITermWTXBranchTab(branch)

	runErr := cmd.Wait()
	result := AgentRunResult{Started: true, Warning: "tmux unavailable; running in current terminal"}
	if runErr != nil {
		return result, fmt.Errorf("worktree command failed: %w", runErr)
	}
	return result, nil
}

func shellCommand(worktreePath string, openShell bool, agentCmd string) *exec.Cmd {
	var cmd *exec.Cmd
	if openShell {
		cmd = exec.Command("/bin/sh", "-lc", "exec \"${SHELL:-/bin/sh}\" -l")
	} else {
		cmd = exec.Command("/bin/sh", "-lc", agentCmd)
	}
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (r *Runner) lockWorktreeForPane(worktreePath string, paneID string, existingLock *WorktreeLock) error {
	if strings.TrimSpace(paneID) == "" {
		return nil
	}
	pid, err := panePID(paneID)
	if err != nil {
		return err
	}
	_, err = r.lockWorktreeForPID(worktreePath, pid, existingLock)
	return err
}

func (r *Runner) lockWorktreeForPID(worktreePath string, pid int, existingLock *WorktreeLock) (*WorktreeLock, error) {
	if existingLock != nil {
		return existingLock, existingLock.RebindPID(pid)
	}
	_, repoRoot, err := requireGitContext(worktreePath)
	if err != nil {
		return nil, err
	}
	return r.lockMgr.AcquireForPID(repoRoot, worktreePath, pid)
}

func (r *Runner) OpenURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return errors.New("no PR URL for selected worktree")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
