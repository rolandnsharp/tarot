//go:build !linux

package main

import "github.com/charmbracelet/ssh"

// Elsewhere the backlog can't be read, so the shuffle runs at its full frame rate.
func rememberConn() ssh.Option { return func(*ssh.Server) error { return nil } }

func linkBacklog(ssh.Session) func() int { return nil }
