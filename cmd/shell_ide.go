package cmd

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func runShell() error {
	if !tmuxAvailable() {
		return errors.New("tmux not available")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command("tmux", "split-window", "-v", "-p", "50", "-c", cwd)
	return cmd.Run()
}

func runIDE(args []string) error {
	if err := ensureConfigReady(); err != nil {
		return err
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	_, ideCmd, err := ensureIDECommandConfigured(cfg)
	if err != nil {
		return err
	}

	var targetPath string
	if len(args) > 0 {
		targetPath = strings.TrimSpace(args[0])
	}
	if targetPath == "" {
		targetPath, _ = os.Getwd()
	}
	// Clean up trailing slashes from empty subpath input
	targetPath = strings.TrimSuffix(targetPath, "/")

	cmd := exec.Command(ideCmd, targetPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
