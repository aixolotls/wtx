package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "wtx error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 1 {
		switch args[1] {
		case "init":
			p := tea.NewProgram(newInitModel())
			return p.Start()
		default:
			return fmt.Errorf("unknown command: %s", args[1])
		}
	}

	exists, err := ConfigExists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("wtx not initialized. run: wtx init")
	}

	p := tea.NewProgram(newModel())
	return p.Start()
}
