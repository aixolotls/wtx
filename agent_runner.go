package main

import (
	"errors"
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
	if !tmuxAvailable() {
		return false, "tmux not available; cannot split pane for agent"
	}
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

	if ok, warn := r.Available(); !ok {
		return AgentRunResult{Started: false, Warning: warn}, nil
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

func (r *AgentRunner) RunShellInWorktree(worktreePath string, branch string, lock *WorktreeLock) (AgentRunResult, error) {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return AgentRunResult{}, errors.New("worktree path required")
	}
	branch = strings.TrimSpace(branch)

	if ok, warn := r.Available(); !ok {
		return AgentRunResult{Started: false, Warning: warn}, nil
	}

	paneID, _ := currentPaneID()
	newPaneID, err := splitShellPane(worktreePath)
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
