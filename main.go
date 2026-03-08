package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
)

var silentMode bool

const defaultPort = "2222"

func main() {
	serve := false
	port := defaultPort
	flag.BoolVar(&silentMode, "silent", false, "disable all audio")
	flag.BoolVar(&serve, "serve", false, "run as SSH server")
	flag.StringVar(&port, "port", defaultPort, "SSH server port")
	flag.Parse()

	if serve {
		runServer(port)
		return
	}

	// Local mode
	m := initialModel()

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)
	_, err := p.Run()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(port string) {
	silentMode = true // no audio on the server
	initAudio()       // no-op, but ensures otoCtx stays nil

	keyPath := ".ssh/tarot_ed25519"
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		os.MkdirAll(".ssh", 0700)
	}

	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%s", port)),
		wish.WithHostKeyPath(keyPath),
		wish.WithMiddleware(
			bm.Middleware(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
				pty, _, _ := sess.Pty()
				m := initialModel()
				m.width = pty.Window.Width
				m.height = pty.Window.Height
				return m, []tea.ProgramOption{tea.WithAltScreen()}
			}),
			lm.Middleware(),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not start SSH server: %v\n", err)
		os.Exit(1)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	fmt.Fprintf(os.Stderr, "Tarot SSH server listening on port %s\n", port)
	fmt.Fprintf(os.Stderr, "Connect with: ssh localhost -p %s\n", port)

	go func() {
		if err := s.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "SSH server error: %v\n", err)
		}
	}()

	<-done
	fmt.Fprintln(os.Stderr, "\nShutting down...")
	s.Shutdown(context.Background())
}
