package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

var silentMode bool

func main() {
	flag.BoolVar(&silentMode, "silent", false, "disable all audio")
	flag.Parse()

	m := initialModel()

	// Set terminal background based on active deck and hide scrollbar
	fmt.Fprintf(os.Stdout, "\033]11;%s\a\033[?30l", deckTheme().Background)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)
	_, err := p.Run()

	// Restore terminal background and scrollbar
	fmt.Fprint(os.Stdout, "\033]111\a\033[?30h")

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
