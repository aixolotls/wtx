package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type WorktreeInfo struct {
	Path               string
	Branch             string
	Available          bool
	PRURL              string
	PRNumber           int
	HasPR              bool
	PRStatus           string
	CIState            PRCIState
	CIDone             int
	CITotal            int
	Approved           bool
	UnresolvedComments int
}

type WorktreeStatus struct {
	GitInstalled bool
	InRepo       bool
	RepoRoot     string
	CWD          string
	BaseRef      string
	Worktrees    []WorktreeInfo
	Orphaned     []WorktreeInfo
	Malformed    []string
	Err          error
}

type WorktreeManager struct {
	cwd            string
	lockMgr        *LockManager
	ghMgr          *GHManager
	baseRefMu      sync.Mutex
	baseRefCache   map[string]cachedBaseRef
	baseRefCacheTTL time.Duration
}

type cachedBaseRef struct {
	value     string
	fetchedAt time.Time
}

func NewWorktreeManager(cwd string, lockMgr *LockManager, ghMgr *GHManager) *WorktreeManager {
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	if lockMgr == nil {
		lockMgr = NewLockManager()
	}
	if ghMgr == nil {
		ghMgr = NewGHManager()
	}
	return &WorktreeManager{
		cwd:            cwd,
		lockMgr:        lockMgr,
		ghMgr:          ghMgr,
		baseRefCache:   map[string]cachedBaseRef{},
		baseRefCacheTTL: 2 * time.Minute,
	}
}

func (m *WorktreeManager) Status() WorktreeStatus {
	status := WorktreeStatus{}
	status.CWD = m.cwd
	gitPath, err := exec.LookPath("git")
	if err != nil {
		status.GitInstalled = false
		return status
	}
	status.GitInstalled = true

	repoRoot, err := gitOutputInDir(m.cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		status.InRepo = false
		return status
	}
	status.InRepo = true
	status.RepoRoot = repoRoot
	status.BaseRef = m.defaultBaseRef(repoRoot, gitPath, false)

	worktrees, malformed, err := listWorktrees(repoRoot, gitPath)
	if err != nil {
		status.Err = err
		return status
	}
	status.Worktrees = worktrees
	status.Malformed = malformed

	orphaned := make([]WorktreeInfo, 0)
	for _, wt := range worktrees {
		exists, err := worktreePathExists(wt.Path)
		if err != nil {
			status.Err = err
			return status
		}
		if !exists {
			wt.Available = false
			for i := range status.Worktrees {
				if status.Worktrees[i].Path == wt.Path {
					status.Worktrees[i].Available = false
					break
				}
			}
			orphaned = append(orphaned, wt)
			continue
		}
		available, err := m.lockMgr.IsAvailable(repoRoot, wt.Path)
		if err != nil {
			status.Err = err
			return status
		}
		for i := range status.Worktrees {
			if status.Worktrees[i].Path == wt.Path {
				status.Worktrees[i].Available = available
				break
			}
		}
	}
	status.Orphaned = orphaned

	return status
}

func (m *WorktreeManager) PRDataForStatus(status WorktreeStatus) map[string]PRData {
	data, _ := m.prDataForStatus(status, false)
	return data
}

func (m *WorktreeManager) PRDataForStatusForce(status WorktreeStatus) map[string]PRData {
	data, _ := m.prDataForStatus(status, true)
	return data
}

func (m *WorktreeManager) PRDataForStatusWithError(status WorktreeStatus, force bool) (map[string]PRData, error) {
	return m.prDataForStatus(status, force)
}

func (m *WorktreeManager) prDataForStatus(status WorktreeStatus, force bool) (map[string]PRData, error) {
	if !status.InRepo || strings.TrimSpace(status.RepoRoot) == "" {
		return map[string]PRData{}, nil
	}
	branches := make([]string, 0, len(status.Worktrees))
	for _, wt := range status.Worktrees {
		b := strings.TrimSpace(wt.Branch)
		if b == "" || b == "detached" {
			continue
		}
		branches = append(branches, b)
	}
	if force {
		return m.ghMgr.PRDataByBranchForce(status.RepoRoot, branches)
	}
	return m.ghMgr.PRDataByBranch(status.RepoRoot, branches)
}

func (m *WorktreeManager) CreateWorktree(branch string) (WorktreeInfo, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return WorktreeInfo{}, errors.New("branch name required")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return WorktreeInfo{}, errors.New("git not installed")
	}

	repoRoot, err := gitOutputInDir(m.cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return WorktreeInfo{}, errors.New("not in a git repository")
	}

	target, err := nextWorktreePath(repoRoot)
	if err != nil {
		return WorktreeInfo{}, err
	}
	lock, err := m.lockMgr.Acquire(repoRoot, target)
	if err != nil {
		return WorktreeInfo{}, err
	}
	defer lock.Release()

	baseRef, err := m.prepareBaseRefForCheckout(repoRoot, gitPath)
	if err != nil {
		return WorktreeInfo{}, err
	}
	cmd := exec.Command(gitPath, "worktree", "add", "-b", branch, target, baseRef)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return WorktreeInfo{}, friendlyBranchCheckoutError(branch, output, err)
	}

	return WorktreeInfo{Path: target, Branch: branch}, nil
}

func (m *WorktreeManager) CreateWorktreeFromBranch(branch string) (WorktreeInfo, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return WorktreeInfo{}, errors.New("branch name required")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return WorktreeInfo{}, errors.New("git not installed")
	}

	repoRoot, err := gitOutputInDir(m.cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return WorktreeInfo{}, errors.New("not in a git repository")
	}

	target, err := nextWorktreePath(repoRoot)
	if err != nil {
		return WorktreeInfo{}, err
	}
	lock, err := m.lockMgr.Acquire(repoRoot, target)
	if err != nil {
		return WorktreeInfo{}, err
	}
	defer lock.Release()

	cmd := exec.Command(gitPath, "worktree", "add", target, branch)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return WorktreeInfo{}, friendlyBranchCheckoutError(branch, output, err)
	}

	return WorktreeInfo{Path: target, Branch: branch}, nil
}

func (m *WorktreeManager) ListLocalBranchesByRecentUse() ([]string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("git not installed")
	}

	repoRoot, err := gitOutputInDir(m.cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return nil, errors.New("not in a git repository")
	}

	cmd := exec.Command(gitPath, "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	branches := make([]string, 0, len(lines))
	for _, raw := range lines {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		branches = append(branches, name)
	}
	return branches, nil
}

func (m *WorktreeManager) DeleteWorktree(path string, force bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("worktree path required")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git not installed")
	}

	repoRoot, err := gitOutputInDir(m.cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return errors.New("not in a git repository")
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	lock, err := m.lockMgr.Acquire(repoRoot, path)
	if err != nil {
		return err
	}
	defer lock.Release()

	cmd := exec.Command(gitPath, args...)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (m *WorktreeManager) CheckoutExistingBranch(worktreePath string, branch string) error {
	worktreePath = strings.TrimSpace(worktreePath)
	branch = strings.TrimSpace(branch)
	if worktreePath == "" {
		return errors.New("worktree path required")
	}
	if branch == "" {
		return errors.New("branch name required")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git not installed")
	}
	cmd := exec.Command(gitPath, "checkout", branch)
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return friendlyBranchCheckoutError(branch, output, err)
	}
	return nil
}

func (m *WorktreeManager) CheckoutNewBranch(worktreePath string, branch string, baseRef string) error {
	worktreePath = strings.TrimSpace(worktreePath)
	branch = strings.TrimSpace(branch)
	baseRef = strings.TrimSpace(baseRef)
	if worktreePath == "" {
		return errors.New("worktree path required")
	}
	if branch == "" {
		return errors.New("branch name required")
	}
	if baseRef == "" {
		baseRef = "main"
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git not installed")
	}
	repoRoot, err := gitOutputInDir(worktreePath, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return errors.New("not in a git repository")
	}
	baseRef, err = m.prepareBaseRefForCheckout(repoRoot, gitPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(gitPath, "checkout", "-b", branch, baseRef)
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return friendlyBranchCheckoutError(branch, output, err)
	}
	return nil
}

func (m *WorktreeManager) AcquireWorktreeLock(worktreePath string) (*WorktreeLock, error) {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return nil, errors.New("worktree path required")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("git not installed")
	}
	repoRoot, err := gitOutputInDir(m.cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return nil, errors.New("not in a git repository")
	}
	return m.lockMgr.Acquire(repoRoot, worktreePath)
}

func (m *WorktreeManager) UnlockWorktree(worktreePath string) error {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return errors.New("worktree path required")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git not installed")
	}
	repoRoot, err := gitOutputInDir(m.cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return errors.New("not in a git repository")
	}
	return m.lockMgr.ForceUnlock(repoRoot, worktreePath)
}

func listWorktrees(repoRoot string, gitPath string) ([]WorktreeInfo, []string, error) {
	cmd := exec.Command(gitPath, "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}
	return parseWorktrees(string(output))
}

func parseWorktrees(output string) ([]WorktreeInfo, []string, error) {
	var worktrees []WorktreeInfo
	var malformed []string
	var current *WorktreeInfo

	lines := strings.Split(output, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "worktree":
			if len(fields) < 2 {
				malformed = append(malformed, line)
				current = nil
				continue
			}
			worktrees = append(worktrees, WorktreeInfo{Path: strings.Join(fields[1:], " ")})
			current = &worktrees[len(worktrees)-1]
		case "branch":
			if current == nil {
				malformed = append(malformed, line)
				continue
			}
			current.Branch = shortBranch(strings.Join(fields[1:], " "))
		case "detached":
			if current == nil {
				malformed = append(malformed, line)
				continue
			}
			if current.Branch == "" {
				current.Branch = "detached"
			}
		default:
			if current == nil {
				malformed = append(malformed, line)
			}
		}
	}

	for i := range worktrees {
		if worktrees[i].Branch == "" {
			worktrees[i].Branch = "detached"
		}
	}
	return worktrees, malformed, nil
}

func shortBranch(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "refs/heads/")
	value = strings.TrimPrefix(value, "refs/remotes/")
	value = strings.TrimPrefix(value, "origin/")
	if value == "" {
		return "detached"
	}
	return value
}

func gitOutputInDir(dir string, path string, args ...string) (string, error) {
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (m *WorktreeManager) defaultBaseRef(repoRoot string, gitPath string, force bool) string {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return "main"
	}
	if !force {
		m.baseRefMu.Lock()
		cached, ok := m.baseRefCache[repoRoot]
		fresh := ok && time.Since(cached.fetchedAt) < m.baseRefCacheTTL && strings.TrimSpace(cached.value) != ""
		m.baseRefMu.Unlock()
		if fresh {
			return cached.value
		}
	}
	if ref, err := githubDefaultBranch(repoRoot); err == nil && strings.TrimSpace(ref) != "" {
		ref = strings.TrimSpace(ref)
		m.baseRefMu.Lock()
		m.baseRefCache[repoRoot] = cachedBaseRef{value: ref, fetchedAt: time.Now()}
		m.baseRefMu.Unlock()
		return ref
	}
	ref, err := gitOutputInDir(repoRoot, gitPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil && ref != "" {
		ref = strings.TrimSpace(ref)
		m.baseRefMu.Lock()
		m.baseRefCache[repoRoot] = cachedBaseRef{value: ref, fetchedAt: time.Now()}
		m.baseRefMu.Unlock()
		return ref
	}
	if _, err := gitOutputInDir(repoRoot, gitPath, "rev-parse", "--verify", "main"); err == nil {
		m.baseRefMu.Lock()
		m.baseRefCache[repoRoot] = cachedBaseRef{value: "main", fetchedAt: time.Now()}
		m.baseRefMu.Unlock()
		return "main"
	}
	if _, err := gitOutputInDir(repoRoot, gitPath, "rev-parse", "--verify", "master"); err == nil {
		m.baseRefMu.Lock()
		m.baseRefCache[repoRoot] = cachedBaseRef{value: "master", fetchedAt: time.Now()}
		m.baseRefMu.Unlock()
		return "master"
	}
	m.baseRefMu.Lock()
	m.baseRefCache[repoRoot] = cachedBaseRef{value: "main", fetchedAt: time.Now()}
	m.baseRefMu.Unlock()
	return "main"
}

func (m *WorktreeManager) prepareBaseRefForCheckout(repoRoot string, gitPath string) (string, error) {
	if err := fetchOrigin(repoRoot, gitPath); err != nil {
		return "", err
	}
	baseRef := strings.TrimSpace(m.defaultBaseRef(repoRoot, gitPath, true))
	if baseRef == "" {
		baseRef = "main"
	}
	return baseRef, nil
}

func githubDefaultBranch(repoRoot string) (string, error) {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(ghPath, "repo", "view", "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", errors.New("github default branch not found")
	}
	return branch, nil
}

func fetchOrigin(repoRoot string, gitPath string) error {
	cmd := exec.Command(gitPath, "remote", "get-url", "origin")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return nil
	}
	fetch := exec.Command(gitPath, "fetch", "origin", "--prune")
	fetch.Dir = repoRoot
	output, err := fetch.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return fmt.Errorf("git fetch origin failed: %s", msg)
		}
		return err
	}
	return nil
}

func friendlyBranchCheckoutError(branch string, output []byte, fallback error) error {
	msg := strings.TrimSpace(string(output))
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "already exists"):
		return fmt.Errorf("branch %q already exists", branch)
	case strings.Contains(lower, "is already checked out at"):
		return fmt.Errorf("branch %q is already checked out in another worktree", branch)
	case strings.Contains(lower, "did not match any file(s) known to git"):
		return fmt.Errorf("branch %q was not found", branch)
	}
	if msg != "" {
		return errors.New(msg)
	}
	return fallback
}

func worktreePathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func nextWorktreePath(repoRoot string) (string, error) {
	base := filepath.Base(repoRoot)
	parent := filepath.Dir(repoRoot)
	worktreeRoot := filepath.Join(parent, base+".wt")
	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(worktreeRoot, fmt.Sprintf("wt.%d", i))
		_, statErr := os.Stat(candidate)
		if errors.Is(statErr, os.ErrNotExist) {
			return candidate, nil
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
	}
	return "", errors.New("no available worktree path")
}
