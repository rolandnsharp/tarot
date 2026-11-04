package main

import (
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
)

// Synchronized output (DEC mode 2026): the terminal holds what it receives between
// the begin and end markers and shows it at once. Bubble Tea writes each frame in a
// single Write, so wrapping every Write keeps a half-arrived shuffle frame from being
// drawn (the horizontal tearing seen over SSH). Terminals without the mode ignore it.
const (
	syncBegin = "\x1b[?2026h"
	syncEnd   = "\x1b[?2026l"
)

func syncWrap(p []byte) []byte {
	out := make([]byte, 0, len(p)+len(syncBegin)+len(syncEnd))
	out = append(out, syncBegin...)
	out = append(out, p...)
	return append(out, syncEnd...)
}

// syncFile keeps the *os.File (Bubble Tea treats a file output as a terminal).
type syncFile struct{ *os.File }

func (f syncFile) Write(p []byte) (int, error) {
	if _, err := f.File.Write(syncWrap(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

type syncWriter struct{ io.Writer }

func (w syncWriter) Write(p []byte) (int, error) {
	if _, err := w.Writer.Write(syncWrap(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// syncOutput replaces the session's output (set by bm.MakeOptions) with a wrapped one.
func syncOutput(sess ssh.Session) tea.ProgramOption {
	if pty, _, ok := sess.Pty(); ok && !sess.EmulatedPty() && pty.Slave != nil {
		return tea.WithOutput(syncFile{pty.Slave})
	}
	return tea.WithOutput(syncWriter{sess})
}
