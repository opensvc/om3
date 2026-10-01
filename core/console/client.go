package console

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type (
	// AttachOptions is what a client attaches a terminal to a console
	// session with.
	AttachOptions struct {
		// URL is the url of the console endpoint, with its ticket.
		URL string

		// TLSConfig is how the endpoint is verified.
		TLSConfig *tls.Config

		// Stdin is where the keystrokes are read from, and Stdout where
		// the output of the session is written. A Stdin that is a
		// terminal is set raw for the time of the session, and its size
		// is the size of the session terminal.
		Stdin  *os.File
		Stdout io.Writer
	}

	// Result is how a console session ended.
	Result struct {
		// Code is the exit code of the command of the session.
		Code int

		// Reason is why the session ended, and Text what the node said
		// about it.
		Reason string
		Text   string
	}

	// wsWriter serializes the writes to a websocket, which takes one
	// writer at a time.
	wsWriter struct {
		sync.Mutex
		conn *websocket.Conn
	}
)

const (
	handshakeTimeout = 10 * time.Second
	writeTimeout     = 10 * time.Second
)

func (t *wsWriter) write(messageType int, b []byte) error {
	t.Lock()
	defer t.Unlock()
	_ = t.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return t.conn.WriteMessage(messageType, b)
}

// Attach opens the console session the url and its ticket name, and
// connects the terminal to it until the session ends.
//
// A session the node ends is a Result. An error is a session that could not
// be opened, or a connection lost: the session is over on the node too, as
// a session ends with its client connection.
func Attach(ctx context.Context, o AttachOptions) (Result, error) {
	var result Result
	dialer := websocket.Dialer{
		TLSClientConfig:  o.TLSConfig,
		HandshakeTimeout: handshakeTimeout,
	}
	conn, resp, err := dialer.DialContext(ctx, o.URL, nil)
	if err != nil {
		return result, dialError(err, resp)
	}
	defer conn.Close()
	w := &wsWriter{conn: conn}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		// Closing the connection is what ends the blocking read below.
		<-ctx.Done()
		_ = conn.Close()
	}()

	restore, err := startTerminal(ctx, o.Stdin, w)
	if err != nil {
		return result, err
	}
	defer restore()

	for {
		messageType, b, err := conn.ReadMessage()
		if err != nil {
			if result.Reason != "" || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return result, nil
			}
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			return result, fmt.Errorf("console connection lost: %w", err)
		}
		switch messageType {
		case websocket.BinaryMessage:
			if _, err := o.Stdout.Write(b); err != nil {
				return result, err
			}
		case websocket.TextMessage:
			m, err := DecodeMessage(b)
			if err != nil {
				continue
			}
			if m.Type == MsgExit {
				result = Result{Code: m.Code, Reason: m.Reason, Text: m.Text}
			}
		}
	}
}

// dialError says why the console endpoint refused the session, which the
// body of its answer holds.
func dialError(err error, resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("console: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if len(b) > 0 {
		return fmt.Errorf("console: %s: %s", resp.Status, string(b))
	}
	return fmt.Errorf("console: %s", resp.Status)
}

// ExitStatus is the non-zero exit status of the command of a session, as an
// error a client command exits with.
//
// A shell exits with the status of the last command typed in it, so it is
// the exit status of the client, as it is of ssh, and no failure to report.
type ExitStatus int

func (t ExitStatus) Error() string {
	return fmt.Sprintf("exit status %d", int(t))
}

// ExitCode is the exit code the client command exits with.
func (t ExitStatus) ExitCode() int {
	return int(t)
}

// ErrNoTerminal is returned by the platforms a terminal can not be attached
// on.
var ErrNoTerminal = errors.New("console: attaching a terminal is not supported on this platform")
