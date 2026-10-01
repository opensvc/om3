package console

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Listener accepts the console connections and hands each of them
	// to a session process of its own.
	Listener struct {
		addr     string
		log      *plog.Logger
		listener net.Listener
		cancel   context.CancelFunc
		wg       sync.WaitGroup

		// unauthenticated counts the sessions that have not accepted
		// their ticket yet.
		unauthenticated atomic.Int32

		// command returns the command serving a session. It is the
		// daemon executable, and something else in the tests.
		command func() *exec.Cmd
	}
)

const (
	// maxUnauthenticated bounds the sessions waiting for their ticket. A
	// connection costs a process before it proved anything, so the ones
	// that prove nothing can not take more than this many at once.
	maxUnauthenticated = 16

	// The descriptors a session process inherits: the connection, and
	// the pipe it closes once its ticket is accepted.
	sessionConnFD = 3
	sessionAuthFD = 4
)

// NewListener returns the console listener of the address.
func NewListener(addr string) *Listener {
	return &Listener{
		addr: addr,
		log:  plog.NewDefaultLogger().Attr("pkg", "daemon/console").WithPrefix("daemon: console: "),
		command: func() *exec.Cmd {
			return exec.Command(os.Args[0], "daemon", "console")
		},
	}
}

// Start listens and accepts until Stop.
func (t *Listener) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", t.addr)
	if err != nil {
		return err
	}
	t.listener = listener
	ctx, t.cancel = context.WithCancel(ctx)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.accept(ctx)
	}()
	t.log.Infof("listening on %s", t.addr)
	return nil
}

// Stop stops accepting. The sessions already handed over are processes of
// their own, and go on.
func (t *Listener) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	if t.listener != nil {
		_ = t.listener.Close()
	}
	t.wg.Wait()
	t.log.Infof("stopped listening on %s", t.addr)
	return nil
}

func (t *Listener) accept(ctx context.Context) {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			t.log.Warnf("accept: %s", err)
			continue
		}
		if err := t.handOver(conn); err != nil {
			t.log.Warnf("%s: %s", conn.RemoteAddr(), err)
		}
		// The session process has its own copy of the connection.
		_ = conn.Close()
	}
}

// handOver starts a session process on the connection.
func (t *Listener) handOver(conn net.Conn) error {
	if n := t.unauthenticated.Load(); n >= maxUnauthenticated {
		return errors.New("refused: too many sessions waiting for their ticket")
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("refused: not a tcp connection")
	}
	connFile, err := tcpConn.File()
	if err != nil {
		return err
	}
	defer connFile.Close()
	authR, authW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer authW.Close()

	cmd := t.command()
	cmd.ExtraFiles = []*os.File{connFile, authW}
	// The session logs by itself: what it would write to the stderr of
	// the daemon would be logged a second time.
	// A session of its own: what is signalled to the daemon and its
	// process group is not signalled to the consoles.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = authR.Close()
		return err
	}
	t.unauthenticated.Add(1)
	go func() {
		// The session closes its end of the pipe once its ticket is
		// accepted, and the pipe is closed with it if it ends before.
		_, _ = io.Copy(io.Discard, authR)
		_ = authR.Close()
		t.unauthenticated.Add(-1)
	}()
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
