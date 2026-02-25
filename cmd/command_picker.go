package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var lookPathFn = exec.LookPath
var isInteractiveTerminalFn = isInteractiveTerminal

type commandPickerType string

const (
	commandPickerCustomValue = "__custom__"
	commandPickerCustomLabel = "(type your choice)"

	commandPickerAgent commandPickerType = "agent"
	commandPickerIDE   commandPickerType = "ide"
)

var agentCandidates = []string{"claude", "codex", "gemini", "opencode"}
var ideCandidates = []string{"code", "cursor", "sublime", "atom"}

func ensureAgentCommandConfigured(cfg Config) (Config, string, error) {
	if v := strings.TrimSpace(cfg.AgentCommand); v != "" {
		return cfg, v, nil
	}
	selected, err := chooseAndSaveCommand(cfg, commandPickerAgent)
	if err != nil {
		return cfg, "", err
	}
	return selected, selected.AgentCommand, nil
}

func ensureIDECommandConfigured(cfg Config) (Config, string, error) {
	if v := strings.TrimSpace(cfg.IDECommand); v != "" {
		return cfg, v, nil
	}
	selected, err := chooseAndSaveCommand(cfg, commandPickerIDE)
	if err != nil {
		return cfg, "", err
	}
	return selected, selected.IDECommand, nil
}

func chooseAndSaveCommand(cfg Config, pickerType commandPickerType) (Config, error) {
	var candidates []string
	var title string
	var placeholder string
	switch pickerType {
	case commandPickerAgent:
		candidates = agentCandidates
		title = "Select AI agent"
		placeholder = "ai agent"
	case commandPickerIDE:
		candidates = ideCandidates
		title = "Select IDE command"
		placeholder = "ide command"
	default:
		return cfg, fmt.Errorf("unsupported picker type %q", pickerType)
	}

	detected := detectInstalledCommands(candidates, lookPathFn)
	if !isInteractiveTerminalFn(os.Stdin) || !isInteractiveTerminalFn(os.Stdout) {
		switch pickerType {
		case commandPickerAgent:
			return cfg, errors.New("agent command not configured; run `wtx config` in an interactive terminal")
		case commandPickerIDE:
			return cfg, errors.New("IDE command not configured; run `wtx config` in an interactive terminal")
		default:
			return cfg, errors.New("command not configured; run `wtx config` in an interactive terminal")
		}
	}

	selected, err := promptCommandSelection(title, detected, placeholder)
	if err != nil {
		return cfg, err
	}
	switch pickerType {
	case commandPickerAgent:
		cfg.AgentCommand = selected
	case commandPickerIDE:
		cfg.IDECommand = selected
	}
	return cfg, SaveConfig(cfg)
}

func detectInstalledCommands(candidates []string, lookPath func(file string) (string, error)) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := lookPath(candidate); err == nil {
			out = append(out, candidate)
		}
	}
	return out
}

func promptCommandSelection(title string, detected []string, placeholder string) (string, error) {
	m := newCommandPickerModel(title, detected, placeholder)
	p := tea.NewProgram(m, tea.WithMouseCellMotion(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	done, ok := finalModel.(commandPickerModel)
	if !ok {
		return "", errors.New("command picker failed")
	}
	if done.cancelled || !done.submitted {
		return "", errors.New("command is required")
	}
	return strings.TrimSpace(done.value), nil
}

func resolvePickedCommand(choice string, custom string) (string, error) {
	custom = strings.TrimSpace(custom)
	if custom != "" {
		return custom, nil
	}
	choice = strings.TrimSpace(choice)
	if choice != "" && choice != commandPickerCustomValue {
		return choice, nil
	}
	return "", errors.New("command is required")
}

type commandPickerMode int

const (
	commandPickerModeSelect commandPickerMode = iota
	commandPickerModeInput
)

type commandPickerModel struct {
	title       string
	placeholder string
	options     []string
	index       int
	mode        commandPickerMode
	input       textinput.Model
	value       string
	errMsg      string
	cancelled   bool
	submitted   bool
}

func newCommandPickerModel(title string, detected []string, placeholder string) commandPickerModel {
	options := make([]string, 0, len(detected)+1)
	for _, cmd := range detected {
		v := strings.TrimSpace(cmd)
		if v == "" {
			continue
		}
		options = append(options, v)
	}
	options = append(options, commandPickerCustomLabel)

	in := textinput.New()
	in.Placeholder = strings.TrimSpace(placeholder)
	in.CharLimit = 200
	in.Width = 40
	return commandPickerModel{
		title:       strings.TrimSpace(title),
		placeholder: strings.TrimSpace(placeholder),
		options:     options,
		input:       in,
	}
}

func (m commandPickerModel) Init() tea.Cmd { return nil }

func (m commandPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode == commandPickerModeSelect {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.cancelled = true
				return m, tea.Quit
			case "up", "k":
				if m.index > 0 {
					m.index--
				}
				return m, nil
			case "down", "j":
				if m.index < len(m.options)-1 {
					m.index++
				}
				return m, nil
			case "enter":
				selection := m.selectedOption()
				if selection == commandPickerCustomLabel {
					m.mode = commandPickerModeInput
					m.input.Focus()
					m.errMsg = ""
					return m, nil
				}
				m.value = strings.TrimSpace(selection)
				m.submitted = m.value != ""
				return m, tea.Quit
			}
			if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
				m.mode = commandPickerModeInput
				m.input.Focus()
				m.input.SetValue(string(msg.Runes))
				m.errMsg = ""
				return m, nil
			}
		}
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			m.mode = commandPickerModeSelect
			m.input.Blur()
			m.errMsg = ""
			return m, nil
		case "enter":
			v := strings.TrimSpace(m.input.Value())
			if v == "" {
				m.errMsg = "Command is required."
				return m, nil
			}
			m.value = v
			m.submitted = true
			return m, tea.Quit
		}
	}
	if m.mode == commandPickerModeInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m commandPickerModel) View() string {
	var b strings.Builder
	if m.title != "" {
		b.WriteString(m.title)
		b.WriteString("\n")
	}
	if m.mode == commandPickerModeSelect {
		for i, option := range m.options {
			line := "  " + actionNormalStyle.Render(option)
			if i == m.index {
				line = "  " + actionSelectedStyle.Render(option)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\nPress enter to select, esc to cancel.\n")
		return b.String()
	}
	b.WriteString("Enter command:\n")
	b.WriteString(inputStyle.Render(m.input.View()))
	b.WriteString("\n")
	if strings.TrimSpace(m.errMsg) != "" {
		b.WriteString(errorStyle.Render(m.errMsg))
		b.WriteString("\n")
	}
	b.WriteString("\nPress enter to save, esc to go back.\n")
	return b.String()
}

func (m commandPickerModel) selectedOption() string {
	if m.index < 0 || m.index >= len(m.options) {
		return ""
	}
	return strings.TrimSpace(m.options[m.index])
}
