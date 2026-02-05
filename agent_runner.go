package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

type AgentRunner struct {
	lockMgr *LockManager
}

func NewAgentRunner() *AgentRunner {
	return &AgentRunner{lockMgr: NewLockManager()}
}

type AgentRunResult struct {
	Started bool
	Warning string
}

func (r *AgentRunner) Available() (bool, string) {
	return true, ""
}

func (r *AgentRunner) RunInWorktree(worktreePath string, branch string, lock *WorktreeLock) (AgentRunResult, error) {
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

	if !tmuxAvailable() {
		lockToRelease, err := r.ensureForegroundLock(worktreePath, lock)
		if err != nil {
			return AgentRunResult{}, err
		}
		defer lockToRelease.Release()
		setITermWTXBranchTab(branch)
		if strings.TrimSpace(agentCmd) == "cd" {
			return AgentRunResult{Started: true}, runShellCommand(worktreePath)
		}
		return AgentRunResult{Started: true}, runAgentCommand(worktreePath, agentCmd)
	}

	paneID, _ := currentPaneID()
	newPaneID, err := splitAgentPane(worktreePath, agentCmd)
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

func (r *AgentRunner) RunShellInWorktree(worktreePath string, branch string, lock *WorktreeLock, ignoreLock bool) (AgentRunResult, error) {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return AgentRunResult{}, errors.New("worktree path required")
	}
	branch = strings.TrimSpace(branch)

	if !tmuxAvailable() {
		if !ignoreLock {
			lockToRelease, err := r.ensureForegroundLock(worktreePath, lock)
			if err != nil {
				return AgentRunResult{}, err
			}
			defer lockToRelease.Release()
		}
		setITermWTXBranchTab(branch)
		return AgentRunResult{Started: true}, runShellCommand(worktreePath)
	}

	paneID, _ := currentPaneID()
	newPaneID, err := splitShellPane(worktreePath)
	if err != nil {
		return AgentRunResult{}, err
	}
	if !ignoreLock {
		if err := r.lockWorktreeForPane(worktreePath, newPaneID, lock); err != nil {
			return AgentRunResult{}, err
		}
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

func (r *AgentRunner) ensureForegroundLock(worktreePath string, existingLock *WorktreeLock) (*WorktreeLock, error) {
	if existingLock != nil {
		return existingLock, nil
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("git not installed")
	}
	repoRoot, err := gitOutputInDir(worktreePath, gitPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	return r.lockMgr.AcquireForPID(repoRoot, worktreePath, os.Getpid())
}

func runShellCommand(worktreePath string) error {
	shellPath := strings.TrimSpace(os.Getenv("SHELL"))
	if shellPath == "" {
		shellPath = "/bin/sh"
	}
	cmd := exec.Command(shellPath)
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runAgentCommand(worktreePath string, agentCmd string) error {
	cmd := exec.Command("/bin/sh", "-lc", agentCmd)
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *AgentRunner) lockWorktreeForPane(worktreePath string, paneID string, existingLock *WorktreeLock) error {
	if strings.TrimSpace(paneID) == "" {
		return nil
	}
	pid, err := panePID(paneID)
	if err != nil {
		return err
	}
	if existingLock != nil {
		return existingLock.RebindPID(pid)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	repoRoot, err := gitOutputInDir(worktreePath, gitPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	_, err = r.lockMgr.AcquireForPID(repoRoot, worktreePath, pid)
	return err
}
