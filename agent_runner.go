package main

import (
	"errors"
	"os/exec"
	"strings"
)

type AgentRunner struct{}

func NewAgentRunner() *AgentRunner {
	return &AgentRunner{}
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

func (r *AgentRunner) RunInWorktree(worktreePath string, branch string) (AgentRunResult, error) {
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
	clearScreen()
	setStatusBanner(renderBanner(branch, worktreePath))
	setITermWTXBranchTab(branch)
	if paneID != "" {
		_ = exec.Command("tmux", "resize-pane", "-t", paneID, "-y", "1").Run()
	}
	if newPaneID != "" {
		_ = exec.Command("tmux", "select-pane", "-t", newPaneID).Run()
	}
	return AgentRunResult{Started: true}, nil
}
