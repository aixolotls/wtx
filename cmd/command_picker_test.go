package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDetectInstalledCommands_PreservesOrder(t *testing.T) {
	candidates := []string{"claude", "codex", "gemini"}
	found := detectInstalledCommands(candidates, func(file string) (string, error) {
		if file == "codex" || file == "gemini" {
			return "/usr/bin/" + file, nil
		}
		return "", errors.New("not found")
	})
	if len(found) != 2 || found[0] != "codex" || found[1] != "gemini" {
		t.Fatalf("unexpected detected commands: %#v", found)
	}
}

func TestResolvePickedCommand_PrefersCustomInput(t *testing.T) {
	got, err := resolvePickedCommand("codex", "claude --model sonnet")
	if err != nil {
		t.Fatalf("resolvePickedCommand: %v", err)
	}
	if got != "claude --model sonnet" {
		t.Fatalf("expected custom command, got %q", got)
	}
}

func TestResolvePickedCommand_UsesDetectedChoice(t *testing.T) {
	got, err := resolvePickedCommand("codex", "")
	if err != nil {
		t.Fatalf("resolvePickedCommand: %v", err)
	}
	if got != "codex" {
		t.Fatalf("expected detected choice, got %q", got)
	}
}

func TestResolvePickedCommand_RejectsEmptyInput(t *testing.T) {
	if _, err := resolvePickedCommand(commandPickerCustomValue, ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestChooseAndSaveCommand_NonInteractiveRequiresConfig(t *testing.T) {
	oldInteractive := isInteractiveTerminalFn
	isInteractiveTerminalFn = func(_ *os.File) bool { return false }
	t.Cleanup(func() {
		isInteractiveTerminalFn = oldInteractive
	})

	cfg, err := chooseAndSaveCommand(Config{}, commandPickerIDE)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(cfg.IDECommand) != "" {
		t.Fatalf("expected IDE command to remain unset, got %q", cfg.IDECommand)
	}
}
