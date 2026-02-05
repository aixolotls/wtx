package main

type Controller struct {
	runner *AgentRunner
}

func NewController() *Controller {
	return &Controller{runner: NewAgentRunner()}
}

func (c *Controller) UseWorktree(worktreePath string, branch string) (AgentRunResult, error) {
	return c.runner.RunInWorktree(worktreePath, branch)
}

func (c *Controller) AgentAvailable() (bool, string) {
	return c.runner.Available()
}
