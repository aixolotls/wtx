package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	mgr               *WorktreeManager
	ctrl              *Controller
	status            WorktreeStatus
	table             table.Model
	ready             bool
	width             int
	height            int
	mode              uiMode
	branchInput       textinput.Model
	newBranchInput    textinput.Model
	spinner           spinner.Model
	errMsg            string
	warnMsg           string
	creatingBranch    string
	deletePath        string
	deleteBranch      string
	actionBranch      string
	actionIndex       int
	actionCreate      bool
	branchOptions     []string
	branchSuggestions []string
	branchIndex       int
	pendingPath       string
	pendingBranch     string
}

func (m model) PendingWorktree() (string, string) {
	return m.pendingPath, m.pendingBranch
}

func newModel() model {
	mgr := NewWorktreeManager("")
	m := model{mgr: mgr, ctrl: NewController()}
	m.table = newTable()
	m.branchInput = newBranchInput()
	m.newBranchInput = newCreateBranchInput()
	m.spinner = newSpinner()
	return m
}

func (m model) Init() tea.Cmd {
	return fetchStatusCmd(m.mgr)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.status = WorktreeStatus(msg)
		m.table.SetRows(worktreeRows(m.status))
		m.ready = true
		return m, nil
	case createWorktreeDoneMsg:
		m.mode = modeList
		m.creatingBranch = ""
		m.actionCreate = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		return m, fetchStatusCmd(m.mgr)
	case spinner.TickMsg:
		if m.mode != modeCreating {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeCreating {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.mode == modeDelete {
			switch msg.String() {
			case "y", "Y":
				force := isOrphanedPath(m.status, m.deletePath)
				if err := m.mgr.DeleteWorktree(m.deletePath, force); err != nil {
					m.errMsg = err.Error()
					m.mode = modeList
					return m, nil
				}
				m.mode = modeList
				m.deletePath = ""
				m.deleteBranch = ""
				m.errMsg = ""
				return m, fetchStatusCmd(m.mgr)
			case "n", "N", "esc":
				m.mode = modeList
				m.deletePath = ""
				m.deleteBranch = ""
				m.errMsg = ""
				return m, nil
			}
			return m, nil
		}
		if m.mode == modeBranchName {
			switch msg.String() {
			case "esc":
				m.mode = modeAction
				m.newBranchInput.Blur()
				m.newBranchInput.SetValue("")
				m.errMsg = ""
				return m, nil
			case "enter":
				branch := strings.TrimSpace(m.newBranchInput.Value())
				if branch == "" {
					m.errMsg = "Branch name required."
					return m, nil
				}
				m.mode = modeCreating
				m.creatingBranch = branch
				m.newBranchInput.Blur()
				m.newBranchInput.SetValue("")
				m.errMsg = ""
				return m, tea.Batch(
					m.spinner.Tick,
					createWorktreeCmd(m.mgr, branch),
				)
			}
			var cmd tea.Cmd
			m.newBranchInput, cmd = m.newBranchInput.Update(msg)
			return m, cmd
		}
		if m.mode == modeAction {
			switch msg.String() {
			case "esc":
				m.mode = modeList
				m.actionIndex = 0
				m.actionBranch = ""
				m.actionCreate = false
				return m, nil
			case "up", "k":
				if m.actionIndex > 0 {
					m.actionIndex--
				}
				return m, nil
			case "down", "j":
				if m.actionIndex < len(currentActionItems(m.actionBranch, m.status.BaseRef, m.actionCreate))-1 {
					m.actionIndex++
				}
				return m, nil
			case "enter":
				if m.actionCreate {
					if m.actionIndex == 0 {
						m.mode = modeBranchName
						m.newBranchInput.SetValue("")
						m.newBranchInput.Focus()
						m.errMsg = ""
						return m, nil
					}
					if m.actionIndex == 1 {
						options, err := availableBranchOptions(m.status, m.mgr)
						if err != nil {
							m.errMsg = err.Error()
							return m, nil
						}
						m.mode = modeBranchPick
						m.branchOptions = options
						m.branchSuggestions = filterBranches(m.branchOptions, "")
						m.branchIndex = 0
						m.branchInput.SetValue("")
						m.branchInput.Focus()
						return m, nil
					}
				}
				if m.actionIndex == 1 {
					m.mode = modeBranchName
					m.newBranchInput.SetValue("")
					m.newBranchInput.Focus()
					m.errMsg = ""
					return m, nil
				}
				if m.actionIndex == 2 {
					options, err := availableBranchOptions(m.status, m.mgr)
					if err != nil {
						m.errMsg = err.Error()
						return m, nil
					}
					m.mode = modeBranchPick
					m.branchOptions = options
					m.branchSuggestions = filterBranches(m.branchOptions, "")
					m.branchIndex = 0
					m.branchInput.SetValue("")
					m.branchInput.Focus()
					return m, nil
				}
				if m.actionIndex == 0 {
					if row, ok := selectedWorktree(m.status, m.table.Cursor()); ok {
						m.errMsg = ""
						m.warnMsg = ""
						if ok, warn := m.ctrl.AgentAvailable(); !ok {
							m.warnMsg = warn
							m.mode = modeList
							return m, nil
						}
						m.pendingPath = row.Path
						m.pendingBranch = row.Branch
						return m, tea.Quit
					}
				}
				m.errMsg = "Not implemented yet."
				m.mode = modeList
				m.actionIndex = 0
				m.actionBranch = ""
				m.actionCreate = false
				return m, nil
			}
			return m, nil
		}
		if m.mode == modeBranchPick {
			switch msg.String() {
			case "esc":
				m.mode = modeAction
				m.branchInput.Blur()
				m.branchSuggestions = nil
				m.branchIndex = 0
				return m, nil
			case "up", "k":
				if m.branchIndex > 0 {
					m.branchIndex--
				}
				return m, nil
			case "down", "j":
				if m.branchIndex < len(m.branchSuggestions)-1 {
					m.branchIndex++
				}
				return m, nil
			case "enter":
				if m.actionCreate {
					branch, ok := selectedBranch(m.branchSuggestions, m.branchIndex)
					if !ok {
						m.errMsg = "Select an existing branch."
						return m, nil
					}
					m.mode = modeCreating
					m.creatingBranch = branch
					m.branchInput.Blur()
					m.branchSuggestions = nil
					m.branchIndex = 0
					m.errMsg = ""
					return m, tea.Batch(
						m.spinner.Tick,
						createWorktreeFromExistingCmd(m.mgr, branch),
					)
				}
				m.errMsg = "Not implemented yet."
				m.mode = modeAction
				m.branchInput.Blur()
				m.branchSuggestions = nil
				m.branchIndex = 0
				return m, nil
			}
			var cmd tea.Cmd
			m.branchInput, cmd = m.branchInput.Update(msg)
			m.branchSuggestions = filterBranches(m.branchOptions, m.branchInput.Value())
			if m.branchIndex >= len(m.branchSuggestions) {
				m.branchIndex = 0
			}
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, fetchStatusCmd(m.mgr)
		case "enter":
			if isCreateRow(m.table.Cursor(), m.status) {
				m.mode = modeAction
				m.actionCreate = true
				m.actionBranch = ""
				m.actionIndex = 0
				m.errMsg = ""
				return m, nil
			}
			if row, ok := selectedWorktree(m.status, m.table.Cursor()); ok {
				if isOrphanedPath(m.status, row.Path) {
					m.errMsg = "Cannot open actions for orphaned worktree."
					return m, nil
				}
				m.mode = modeAction
				m.actionCreate = false
				m.actionBranch = row.Branch
				m.actionIndex = 0
				m.errMsg = ""
				return m, nil
			}
		case "d":
			if row, ok := selectedWorktree(m.status, m.table.Cursor()); ok {
				m.mode = modeDelete
				m.deletePath = row.Path
				m.deleteBranch = row.Branch
				m.errMsg = ""
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(bannerStyle.Render("WTX"))
	b.WriteString("\n\n")

	if !m.ready {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if !m.status.GitInstalled {
		b.WriteString(errorStyle.Render("Git not installed."))
		b.WriteString("\n")
		b.WriteString("Install git to use wtx.\n")
		b.WriteString("\n")
		b.WriteString("Press q to quit.\n")
		return b.String()
	}

	if !m.status.InRepo {
		b.WriteString(errorStyle.Render("Not inside a git repository."))
		b.WriteString("\n")
		if m.status.CWD != "" {
			b.WriteString(fmt.Sprintf("CWD: %s\n", m.status.CWD))
		}
		b.WriteString("\n")
		b.WriteString("Press q to quit.\n")
		return b.String()
	}

	if m.mode == modeAction {
		title := "Worktree actions:"
		if m.actionCreate {
			title = "New worktree actions:"
		}
		b.WriteString(title + "\n")
		for i, item := range currentActionItems(m.actionBranch, m.status.BaseRef, m.actionCreate) {
			line := "  " + actionNormalStyle.Render(item)
			if i == m.actionIndex {
				line = "  " + actionSelectedStyle.Render(item)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\nPress enter to select, esc to cancel.\n")
		return b.String()
	}
	if m.mode == modeBranchName {
		title := "New branch name:"
		if m.actionCreate {
			title = "New worktree branch:"
		}
		b.WriteString(title + "\n")
		b.WriteString(inputStyle.Render(m.newBranchInput.View()))
		b.WriteString("\n")
		if m.errMsg != "" {
			b.WriteString(errorStyle.Render(m.errMsg))
			b.WriteString("\n")
		}
		b.WriteString("\nPress enter to create, esc to cancel.\n")
		return b.String()
	}
	if m.mode == modeBranchPick {
		b.WriteString("Choose an existing branch:\n")
		b.WriteString(inputStyle.Render(m.branchInput.View()))
		b.WriteString("\n")
		for i, suggestion := range m.branchSuggestions {
			line := "  " + actionNormalStyle.Render(suggestion)
			if i == m.branchIndex {
				line = "  " + actionSelectedStyle.Render(suggestion)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\nPress enter to select, esc to cancel.\n")
		return b.String()
	}
	if m.mode == modeDelete {
		b.WriteString("Delete worktree:\n")
		b.WriteString(fmt.Sprintf("%s\n", m.deleteBranch))
		b.WriteString(fmt.Sprintf("%s\n", m.deletePath))
		b.WriteString("\nAre you sure? (y/N)\n")
		return b.String()
	}
	b.WriteString(baseStyle.Render(m.table.View()))
	if m.status.Err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.status.Err)))
		b.WriteString("\n")
	}
	if m.errMsg != "" {
		b.WriteString(errorStyle.Render(m.errMsg))
		b.WriteString("\n")
	}
	if m.mode == modeCreating {
		b.WriteString("\n")
		b.WriteString(m.spinner.View())
		b.WriteString(" Creating ")
		b.WriteString(branchStyle.Render(m.creatingBranch))
		b.WriteString("...\n")
	}
	if m.warnMsg != "" {
		b.WriteString(warnStyle.Render(m.warnMsg))
		b.WriteString("\n")
	}
	if len(m.status.Malformed) > 0 {
		b.WriteString("\nMalformed entries:\n")
		for _, line := range m.status.Malformed {
			b.WriteString(" - ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	selectedPath := currentWorktreePath(m.status, m.table.Cursor())
	if selectedPath != "" {
		b.WriteString("\n")
		b.WriteString(secondaryStyle.Render(selectedPath))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	help := "Press q to quit."
	if m.mode == modeCreating {
		help = "Creating worktree..."
	} else if isCreateRow(m.table.Cursor(), m.status) {
		help = "Press enter for actions, q to quit."
	} else if _, ok := selectedWorktree(m.status, m.table.Cursor()); ok {
		help = "Press enter for actions, d to delete, q to quit."
	}
	b.WriteString(help + "\n")
	return b.String()
}

type statusMsg WorktreeStatus
type createWorktreeDoneMsg struct {
	created WorktreeInfo
	err     error
}

func fetchStatusCmd(mgr *WorktreeManager) tea.Cmd {
	return func() tea.Msg {
		return statusMsg(mgr.Status())
	}
}

func createWorktreeCmd(mgr *WorktreeManager, branch string) tea.Cmd {
	return func() tea.Msg {
		created, err := mgr.CreateWorktree(branch)
		return createWorktreeDoneMsg{created: created, err: err}
	}
}

func createWorktreeFromExistingCmd(mgr *WorktreeManager, branch string) tea.Cmd {
	return func() tea.Msg {
		created, err := mgr.CreateWorktreeFromBranch(branch)
		return createWorktreeDoneMsg{created: created, err: err}
	}
}

func newTable() table.Model {
	columns := []table.Column{
		{Title: "Branch", Width: 28},
		{Title: "Status", Width: 10},
		{Title: "PR", Width: 6},
		{Title: "CI", Width: 6},
		{Title: "Approved", Width: 9},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(tableStyles())
	return t
}

func worktreeRows(status WorktreeStatus) []table.Row {
	if !status.InRepo {
		return nil
	}
	orphaned := make(map[string]bool, len(status.Orphaned))
	for _, wt := range status.Orphaned {
		orphaned[wt.Path] = true
	}
	rows := make([]table.Row, 0, len(status.Worktrees))
	for _, wt := range status.Worktrees {
		label := wt.Branch
		if orphaned[wt.Path] {
			label = fmt.Sprintf("%s (orphaned)", wt.Branch)
		}
		statusLabel := "Free"
		if strings.Contains(strings.ToLower(wt.Branch), "main") {
			statusLabel = "In use"
		}
		pr := greenCheck()
		ci := redX()
		approved := greenCheck()
		rows = append(rows, table.Row{label, statusLabel, pr, ci, approved})
	}
	rows = append(rows, table.Row{"+ New worktree", "", "", "", ""})
	return rows
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderBottom(false).
		Bold(true).
		Foreground(lipgloss.Color("15")) // primary text
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("15")). // primary text
		Background(lipgloss.Color("8")).  // selected background
		Bold(true)
	s.Cell = s.Cell.
		Foreground(lipgloss.Color("251")) // secondary text
	return s
}

var (
	baseStyle   = lipgloss.NewStyle()
	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF7DB")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	secondaryStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	actionNormalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("251"))
	actionSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8")).Bold(true)
	branchStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	warnStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	inputStyle          = lipgloss.NewStyle().
				Padding(0, 1)
)

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

type uiMode int

const (
	modeList uiMode = iota
	modeCreating
	modeDelete
	modeAction
	modeBranchName
	modeBranchPick
)

func newBranchInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "branch name"
	ti.CharLimit = 200
	ti.Width = 40
	return ti
}

func newCreateBranchInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "feature/my-branch"
	ti.CharLimit = 200
	ti.Width = 40
	return ti
}

func isCreateRow(cursor int, status WorktreeStatus) bool {
	if !status.InRepo {
		return false
	}
	if cursor < 0 {
		return false
	}
	return cursor == len(status.Worktrees)
}

func selectedWorktree(status WorktreeStatus, cursor int) (WorktreeInfo, bool) {
	if !status.InRepo {
		return WorktreeInfo{}, false
	}
	if cursor < 0 || cursor >= len(status.Worktrees) {
		return WorktreeInfo{}, false
	}
	return status.Worktrees[cursor], true
}

func isOrphanedPath(status WorktreeStatus, path string) bool {
	for _, wt := range status.Orphaned {
		if wt.Path == path {
			return true
		}
	}
	return false
}

func actionItems(branch string, baseRef string) []string {
	base := strings.TrimSpace(baseRef)
	if base == "" {
		base = "main"
	}
	return []string{
		"Use " + branchStyle.Render(branch),
		"Checkout new branch from " + branchStyle.Render(base),
		"Choose an existing branch",
		"Open shell here",
	}
}

func createActionItems(baseRef string) []string {
	base := strings.TrimSpace(baseRef)
	if base == "" {
		base = "main"
	}
	return []string{
		"Checkout new branch from " + branchStyle.Render(base),
		"Choose an existing branch",
	}
}

func currentActionItems(branch string, baseRef string, create bool) []string {
	if create {
		return createActionItems(baseRef)
	}
	return actionItems(branch, baseRef)
}

func currentWorktreePath(status WorktreeStatus, cursor int) string {
	if !status.InRepo {
		return ""
	}
	if cursor < 0 || cursor >= len(status.Worktrees) {
		return ""
	}
	return status.Worktrees[cursor].Path
}

func greenCheck() string {
	return "✓"
}

func redX() string {
	return "✗"
}

func uniqueBranches(status WorktreeStatus) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(status.Worktrees)+1)
	for _, wt := range status.Worktrees {
		name := strings.TrimSpace(wt.Branch)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if !seen["main"] {
		out = append(out, "main")
	}
	return out
}

func filterBranches(options []string, query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return options
	}
	out := make([]string, 0, len(options))
	for _, opt := range options {
		if strings.Contains(strings.ToLower(opt), q) {
			out = append(out, opt)
		}
	}
	return out
}

func availableBranchOptions(status WorktreeStatus, mgr *WorktreeManager) ([]string, error) {
	options, err := mgr.ListLocalBranchesByRecentUse()
	if err != nil {
		return nil, err
	}
	inUse := make(map[string]bool, len(status.Worktrees))
	for _, wt := range status.Worktrees {
		name := strings.TrimSpace(wt.Branch)
		if name == "" {
			continue
		}
		inUse[name] = true
	}
	filtered := make([]string, 0, len(options))
	for _, opt := range options {
		if inUse[opt] {
			continue
		}
		filtered = append(filtered, opt)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no available branches (all branches currently in use)")
	}
	return filtered, nil
}

func selectedBranch(suggestions []string, index int) (string, bool) {
	if index < 0 || index >= len(suggestions) {
		return "", false
	}
	value := strings.TrimSpace(suggestions[index])
	return value, value != ""
}

func nextAutoBranchName(status WorktreeStatus) string {
	base := strings.TrimSpace(shortBranch(status.BaseRef))
	if base == "" || base == "detached" {
		base = "main"
	}
	base = strings.ReplaceAll(base, "/", "-")
	base = strings.ReplaceAll(base, " ", "-")
	return fmt.Sprintf("wt/%s-%d", base, time.Now().UnixNano())
}

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	return s
}
