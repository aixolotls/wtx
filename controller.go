package main

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

type Controller struct {
	runner *AgentRunner
}

func NewController() *Controller {
	return &Controller{runner: NewAgentRunner()}
}

func (c *Controller) UseWorktree(worktreePath string, branch string, lock *WorktreeLock) (AgentRunResult, error) {
	return c.runner.RunInWorktree(worktreePath, branch, lock)
}

func (c *Controller) OpenShellInWorktree(worktreePath string, branch string, lock *WorktreeLock, ignoreLock bool) (AgentRunResult, error) {
	return c.runner.RunShellInWorktree(worktreePath, branch, lock, ignoreLock)
}

func (c *Controller) AgentAvailable() (bool, string) {
	return c.runner.Available()
}

func (c *Controller) OpenURL(url string) error {
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
