package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	sshMode := flag.Bool("ssh", false, "run as an SSH server")
	sshHost := flag.String("host", "localhost", "SSH server host")
	sshPort := flag.String("port", "2222", "SSH server port")
	flag.Parse()

	if *sshMode {
		runSSHServer(*sshHost, *sshPort)
		return
	}

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
