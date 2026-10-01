//go:build unix

package console

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// startTerminal forwards the keystrokes read from stdin to the session, and
// the size of the terminal when it changes. It returns the function giving
// the terminal back as it was found.
//
// The reads are interruptible: a read left blocked on the terminal after the
// session would take the next keystroke of whoever reads it then, as a TUI
// resuming does.
func startTerminal(ctx context.Context, stdin *os.File, w *wsWriter) (func(), error) {
	if stdin == nil {
		return func() {}, nil
	}
	fd := int(stdin.Fd())
	isTerminal := term.IsTerminal(fd)

	var state *term.State
	if isTerminal {
		var err error
		if state, err = term.MakeRaw(fd); err != nil {
			return nil, err
		}
	}

	wakeR, wakeW, err := os.Pipe()
	if err != nil {
		if state != nil {
			_ = term.Restore(fd, state)
		}
		return nil, err
	}
	done := make(chan struct{})

	sendSize := func() {
		if cols, rows, err := term.GetSize(fd); err == nil {
			_ = w.write(websocket.TextMessage, Message{Type: MsgResize, Cols: uint16(cols), Rows: uint16(rows)}.Encode())
		}
	}
	winch := make(chan os.Signal, 1)
	if isTerminal {
		sendSize()
		signal.Notify(winch, syscall.SIGWINCH)
	}

	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		fds := []unix.PollFd{
			{Fd: int32(fd), Events: unix.POLLIN},
			{Fd: int32(wakeR.Fd()), Events: unix.POLLIN},
		}
		for {
			fds[0].Revents, fds[1].Revents = 0, 0
			if _, err := unix.Poll(fds, -1); err != nil {
				if err == unix.EINTR {
					continue
				}
				return
			}
			if fds[1].Revents != 0 {
				return
			}
			if fds[0].Revents == 0 {
				continue
			}
			n, err := unix.Read(fd, buf)
			if n > 0 {
				if err := w.write(websocket.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
			if err != nil && err != unix.EINTR && err != unix.EAGAIN {
				return
			}
			if n == 0 && err == nil {
				// The end of a stdin that is not a terminal: nothing
				// more to send, and the session goes on until its
				// command ends.
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-winch:
				sendSize()
			}
		}
	}()

	return func() {
		signal.Stop(winch)
		_, _ = wakeW.Write([]byte{0})
		<-done
		_ = wakeR.Close()
		_ = wakeW.Close()
		if state != nil {
			_ = term.Restore(fd, state)
		}
	}, nil
}
