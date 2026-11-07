package main

import (
	"net"

	"github.com/charmbracelet/ssh"
	"golang.org/x/sys/unix"
)

type tcpConnKey struct{}

// rememberConn keeps each client's TCP connection in its SSH context, so a session
// can see how far behind the link is.
func rememberConn() ssh.Option {
	return ssh.WrapConn(func(ctx ssh.Context, c net.Conn) net.Conn {
		ctx.SetValue(tcpConnKey{}, c)
		return c
	})
}

// linkBacklog returns a function reporting how many bytes sent to this client are not
// yet acknowledged by it (SIOCOUTQ), or nil if that can't be measured. The shuffle
// uses it to draw only as many frames as the link is actually delivering, instead of
// queueing frames that arrive late and in bursts.
func linkBacklog(sess ssh.Session) func() int {
	tc, ok := sess.Context().Value(tcpConnKey{}).(*net.TCPConn)
	if !ok {
		return nil
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return nil
	}
	return func() int {
		n := 0
		raw.Control(func(fd uintptr) {
			n, _ = unix.IoctlGetInt(int(fd), unix.SIOCOUTQ)
		})
		return n
	}
}
