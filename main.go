package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Set terminal background to black (OSC 11)
	fmt.Fprint(os.Stdout, "\033]11;#0a0510\a")

	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
	)
	_, err := p.Run()

	// Restore original terminal background (OSC 111)
	fmt.Fprint(os.Stdout, "\033]111\a")

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
