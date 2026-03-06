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

	// Set terminal background to dark purple and hide scrollbar
	fmt.Fprint(os.Stdout, "\033]11;#0a0510\a\033[?30l")

	p := tea.NewProgram(
		initialModel(),
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
