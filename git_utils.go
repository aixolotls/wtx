package main

import (
	"errors"
	"os/exec"
	"strings"
)

var errGitNotInstalled = errors.New("git not installed")
var errNotInGitRepository = errors.New("not in a git repository")

func gitPath() (string, error) {
	return exec.LookPath("git")
}

func requireGitPath() (string, error) {
	path, err := gitPath()
	if err != nil {
		return "", errGitNotInstalled
	}
	return path, nil
}

func repoRootForDir(dir string, gitBin string) (string, error) {
	repoRoot, err := gitOutputInDir(dir, gitBin, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return "", errNotInGitRepository
	}
	return repoRoot, nil
}

func requireGitContext(dir string) (string, string, error) {
	gitBin, err := requireGitPath()
	if err != nil {
		return "", "", err
	}
	repoRoot, err := repoRootForDir(dir, gitBin)
	if err != nil {
		return "", "", err
	}
	return gitBin, repoRoot, nil
}
