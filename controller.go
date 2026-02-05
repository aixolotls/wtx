package main

type Controller struct {
	runner *AgentRunner
}

func NewController() *Controller {
	return &Controller{runner: NewAgentRunner()}
}

func (c *Controller) UseWorktree(worktreePath string, branch string, lock *WorktreeLock) (AgentRunResult, error) {
	return c.runner.RunInWorktree(worktreePath, branch, lock)
}

func (c *Controller) OpenShellInWorktree(worktreePath string, branch string, lock *WorktreeLock) (AgentRunResult, error) {
	return c.runner.RunShellInWorktree(worktreePath, branch, lock)
}

func (c *Controller) AgentAvailable() (bool, string) {
	return c.runner.Available()
}
