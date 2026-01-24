package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/manifoldco/promptui"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "claudex error:", err)
		os.Exit(exitCode(err))
	}
}

func run(args []string) error {
	if len(args) > 1 {
		return usageError()
	}

	repoInfo, ok := detectRepoInfo()
	if !ok {
		if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
			setITermGrayTab()
		}
		return nil
	}

	lock, err := acquireWorktree(repoInfo)
	if err != nil {
		return err
	}
	defer lock.Release()
	lock.StartToucher()
	defer lock.StopToucher()

	fmt.Fprintln(os.Stdout, "using worktree at", lock.WorktreePath)

	title := fmt.Sprintf("[%s][%s]", repoInfo.Name, lock.Branch)
	setTitle(title)
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		setITermBlueTab()
	}
	waitForInterrupt()
	return nil
}

func usageError() error {
	fmt.Fprintln(os.Stderr, "usage: claudex")
	return errors.New("invalid arguments")
}

func setTitle(title string) {
	osc0 := "\x1b]0;" + title + "\x07"
	osc2 := "\x1b]2;" + title + "\x07"
	fmt.Fprint(os.Stdout, osc0, osc2)
}

func setITermBlueTab() {
	fmt.Fprint(os.Stdout,
		"\x1b]1337;SetTabColor=rgb:00/00/ff\x07",
		"\x1b]6;1;bg;red;brightness;0\x07",
		"\x1b]6;1;bg;green;brightness;0\x07",
		"\x1b]6;1;bg;blue;brightness;255\x07",
	)
}

func setITermGrayTab() {
	fmt.Fprint(os.Stdout,
		"\x1b]1337;SetTabColor=rgb:66/66/66\x07",
		"\x1b]6;1;bg;red;brightness;102\x07",
		"\x1b]6;1;bg;green;brightness;102\x07",
		"\x1b]6;1;bg;blue;brightness;102\x07",
	)
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

type repoInfo struct {
	Name   string
	Branch string
	Path   string
}

type worktreeInfo struct {
	Path   string
	Branch string
}

type worktreeOption struct {
	Info      worktreeInfo
	Available bool
}

type worktreeOptionView struct {
	Info        worktreeInfo
	Unavailable string
}

type worktreeLock struct {
	Path         string
	WorktreePath string
	RepoRoot     string
	Branch       string
	stopCh       chan struct{}
	stopOnce     sync.Once
}

func detectRepoInfo() (repoInfo, bool) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return repoInfo{}, false
	}

	cwd, err := os.Getwd()
	if err != nil {
		return repoInfo{}, false
	}

	topLevel, err := gitOutputInDir(cwd, gitPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return repoInfo{}, false
	}

	branch, err := gitOutputInDir(cwd, gitPath, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || branch == "" {
		branch, err = gitOutputInDir(cwd, gitPath, "rev-parse", "--short", "HEAD")
		if err != nil || branch == "" {
			branch = "detached"
		}
	}

	name := filepath.Base(topLevel)
	if name == "" || name == string(filepath.Separator) {
		name = "repo"
	}

	return repoInfo{Name: name, Branch: branch, Path: topLevel}, true
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

func acquireWorktree(info repoInfo) (*worktreeLock, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}

	for {
		worktrees, err := listWorktrees(info.Path, gitPath)
		if err != nil {
			return nil, err
		}

		options := make([]worktreeOption, 0, len(worktrees))
		for _, wt := range worktrees {
			available, err := isWorktreeAvailable(info.Path, wt.Path)
			if err != nil {
				return nil, err
			}
			options = append(options, worktreeOption{Info: wt, Available: available})
		}

		if !hasAvailable(options) {
			create, err := promptYesNo("No available worktrees. Create new? [y/N]: ")
			if err != nil {
				return nil, err
			}
			if !create {
				return nil, errors.New("no available worktrees")
			}
			if err := createWorktree(info.Path, gitPath); err != nil {
				return nil, err
			}
			continue
		}

		available, unavailableList := splitWorktrees(options)
		chosen, err := promptSelectWorktree(available, unavailableList)
		if err != nil {
			return nil, err
		}

		lock, err := lockWorktree(info.Path, chosen)
		if err != nil {
			continue
		}
		return lock, nil
	}
}

func listWorktrees(repoRoot string, gitPath string) ([]worktreeInfo, error) {
	cmd := exec.Command(gitPath, "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var worktrees []worktreeInfo
	var current *worktreeInfo

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "worktree":
			worktrees = append(worktrees, worktreeInfo{Path: strings.Join(fields[1:], " ")})
			current = &worktrees[len(worktrees)-1]
		case "branch":
			if current != nil {
				current.Branch = shortBranch(strings.Join(fields[1:], " "))
			}
		case "detached":
			if current != nil && current.Branch == "" {
				current.Branch = "detached"
			}
		}
	}

	for i := range worktrees {
		if worktrees[i].Branch == "" {
			worktrees[i].Branch = "detached"
		}
	}

	return worktrees, nil
}

func shortBranch(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return ref
}

func promptYesNo(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stdout, prompt)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

func promptSelectWorktree(available []worktreeInfo, unavailableList string) (worktreeInfo, error) {
	funcMap := template.FuncMap{}
	for key, value := range promptui.FuncMap {
		funcMap[key] = value
	}

	items := make([]worktreeOptionView, 0, len(available))
	for _, wt := range available {
		items = append(items, worktreeOptionView{
			Info:        wt,
			Unavailable: unavailableList,
		})
	}

	templates := &promptui.SelectTemplates{
		Active:   "{{ cyan \"▸\" }} {{ .Info.Path }} [{{ .Info.Branch }}]",
		Inactive: "  {{ .Info.Path }} [{{ .Info.Branch }}]",
		Selected: "{{ cyan \"✔\" }} {{ .Info.Path }} [{{ .Info.Branch }}]",
		Details:  "{{ if .Unavailable }}\nIn use worktrees:\n{{ faint .Unavailable }}{{ end }}",
		FuncMap:  funcMap,
	}

	for {
		selectPrompt := promptui.Select{
			Label:     "Choose worktree",
			Items:     items,
			Templates: templates,
		}
		index, _, err := selectPrompt.Run()
		if err != nil {
			return worktreeInfo{}, err
		}
		return available[index], nil
	}
}

func hasAvailable(options []worktreeOption) bool {
	for _, option := range options {
		if option.Available {
			return true
		}
	}
	return false
}

func splitWorktrees(options []worktreeOption) ([]worktreeInfo, string) {
	available := make([]worktreeInfo, 0, len(options))
	var unavailable []worktreeInfo
	for _, option := range options {
		if option.Available {
			available = append(available, option.Info)
			continue
		}
		unavailable = append(unavailable, option.Info)
	}
	return available, formatUnavailableList(unavailable)
}

func formatUnavailableList(items []worktreeInfo) string {
	if len(items) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString("  ")
		builder.WriteString(item.Path)
		builder.WriteString(" [")
		builder.WriteString(item.Branch)
		builder.WriteString("] (in use)\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func createWorktree(repoRoot string, gitPath string) error {
	baseRef := defaultBaseRef(repoRoot, gitPath)
	branch, err := promptBranchName(baseRef)
	if err != nil {
		return err
	}
	target, err := nextWorktreePath(repoRoot)
	if err != nil {
		return err
	}
	cmd := exec.Command(gitPath, "worktree", "add", "-b", branch, target, baseRef)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func promptBranchName(baseRef string) (string, error) {
	label := fmt.Sprintf("New branch name (from %s)", baseRef)
	prompt := promptui.Prompt{Label: label}
	value, err := prompt.Run()
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("branch name required")
	}
	return value, nil
}

func defaultBaseRef(repoRoot string, gitPath string) string {
	ref, err := gitOutputInDir(repoRoot, gitPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil && ref != "" {
		return ref
	}
	ref, err = gitOutputInDir(repoRoot, gitPath, "symbolic-ref", "--short", "HEAD")
	if err == nil && ref != "" {
		return ref
	}
	return "HEAD"
}

func nextWorktreePath(repoRoot string) (string, error) {
	parent := filepath.Dir(repoRoot)
	base := filepath.Base(repoRoot)
	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(parent, fmt.Sprintf("%s.wt.%d", base, i))
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

func isWorktreeAvailable(repoRoot string, worktreePath string) (bool, error) {
	lockPath, err := worktreeLockPath(repoRoot, worktreePath)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(lockPath)
	if err == nil {
		if time.Since(info.ModTime()) < 10*time.Second {
			return false, nil
		}
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return false, err
}

func lockWorktree(repoRoot string, wt worktreeInfo) (*worktreeLock, error) {
	lockPath, err := worktreeLockPath(repoRoot, wt.Path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}

	payload, err := lockPayload(repoRoot, wt.Path)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		if _, werr := file.Write(payload); werr != nil {
			file.Close()
			_ = os.Remove(lockPath)
			return nil, werr
		}
		_ = file.Close()
		return newWorktreeLock(lockPath, wt, repoRoot), nil
	}

	if !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	info, statErr := os.Stat(lockPath)
	if statErr != nil {
		return nil, statErr
	}
	if time.Since(info.ModTime()) < 10*time.Second {
		return nil, errors.New("worktree locked")
	}

	tmpPath := lockPath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, lockPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	return newWorktreeLock(lockPath, wt, repoRoot), nil
}

func lockPayload(repoRoot string, worktreePath string) ([]byte, error) {
	ownerID := buildOwnerID()
	data := map[string]any{
		"pid":           os.Getpid(),
		"owner_id":      ownerID,
		"worktree_path": worktreePath,
		"repo_root":     repoRoot,
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
	}
	return json.Marshal(data)
}

func newWorktreeLock(path string, wt worktreeInfo, repoRoot string) *worktreeLock {
	return &worktreeLock{
		Path:         path,
		WorktreePath: wt.Path,
		RepoRoot:     repoRoot,
		Branch:       wt.Branch,
		stopCh:       make(chan struct{}),
	}
}

func (l *worktreeLock) StartToucher() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				_ = os.Chtimes(l.Path, now, now)
			case <-l.stopCh:
				return
			}
		}
	}()
}

func (l *worktreeLock) StopToucher() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
}

func (l *worktreeLock) Release() {
	_ = os.Remove(l.Path)
}

func worktreeLockPath(repoRoot string, worktreePath string) (string, error) {
	repoRootReal, err := realPath(repoRoot)
	if err != nil {
		return "", err
	}
	worktreeReal, err := realPath(worktreePath)
	if err != nil {
		return "", err
	}

	repoID := hashString(repoRootReal)
	worktreeID := hashString(repoID + ":" + worktreeReal)

	lockDir := filepath.Join(os.Getenv("HOME"), ".claudex", "locks")
	return filepath.Join(lockDir, worktreeID+".lock"), nil
}

func realPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func buildOwnerID() string {
	name := os.Getenv("USER")
	if name == "" {
		if u, err := user.Current(); err == nil {
			name = u.Username
		}
	}
	host, _ := os.Hostname()
	if name == "" && host == "" {
		return "unknown"
	}
	if host == "" {
		return name
	}
	if name == "" {
		return host
	}
	return name + "@" + host
}

func waitForInterrupt() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}
