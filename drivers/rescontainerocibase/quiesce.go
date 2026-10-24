package rescontainerocibase

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const defaultQuiesceTimeout = 30 * time.Second

// Quiesce implements resource.Quiescer: it suspends the writes of the
// container for a run of a sync resource, as the quiesce keyword says. pause
// freezes its processes with the engine's pause. A command runs in the
// container, to its end, or, with quiesce_ready, as a session held open
// until the resume, as a database client holding a lock.
func (t *BT) Quiesce(ctx context.Context) (func(context.Context) error, error) {
	switch t.QuiesceCmd {
	case "":
		return nil, nil
	case "pause":
		pauser, ok := t.executer.(ExecutePauser)
		if !ok {
			return nil, fmt.Errorf("quiesce: the engine of %s does not pause containers", t.RID())
		}
		t.Log().Infof("quiesce: pause the container")
		if err := pauser.Pause(ctx); err != nil {
			return nil, fmt.Errorf("quiesce: pause: %w", err)
		}
		return func(ctx context.Context) error {
			t.Log().Infof("resume: unpause the container")
			if err := pauser.Unpause(ctx); err != nil {
				return fmt.Errorf("resume: unpause: %w", err)
			}
			return nil
		}, nil
	}
	if t.executer == nil {
		return nil, errors.New("quiesce: undefined executer")
	}
	timeout := defaultQuiesceTimeout
	if t.QuiesceTimeout != nil && *t.QuiesceTimeout > 0 {
		timeout = *t.QuiesceTimeout
	}
	// The session outlives the action context until the resume: if om
	// ends first, its stdin closes, the command exits, and a lock it held
	// drops by itself.
	sctx, cancel := context.WithCancel(context.Background())
	// An os pipe, which the command reads itself: with another reader,
	// the wait for the command would wait for its stdin to close too.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("quiesce: %w", err)
	}
	cmd, err := t.executer.EncapCmd(sctx, []string{"sh", "-c", t.QuiesceCmd}, nil, stdinR)
	if err != nil {
		cancel()
		_ = stdinR.Close()
		_ = stdinW.Close()
		return nil, fmt.Errorf("quiesce: %w", err)
	}
	session := newQuiesceSession(cmd, stdinR, stdinW, cancel, t.QuiesceReady, t.Log().Debugf)
	t.Log().Infof("quiesce: run %s", t.QuiesceCmd)
	if err := session.start(t.QuiesceInput); err != nil {
		return nil, fmt.Errorf("quiesce: %w", err)
	}
	if t.QuiesceReady == "" {
		if err := session.end("", timeout); err != nil {
			return nil, fmt.Errorf("quiesce: %w", err)
		}
		return nil, nil
	}
	if err := session.waitReady(timeout); err != nil {
		return nil, fmt.Errorf("quiesce: %w", err)
	}
	return func(ctx context.Context) error {
		t.Log().Infof("resume: end the quiesce session")
		if err := session.end(t.ResumeInput, timeout); err != nil {
			return fmt.Errorf("resume: %w", err)
		}
		return nil
	}, nil
}

// quiesceSession is a command quiescing writes, fed on its stdin, its stdout
// read for the line saying it is ready.
type quiesceSession struct {
	cmd    *exec.Cmd
	stdinR io.Closer
	stdin  io.WriteCloser
	cancel context.CancelFunc
	ready  string
	logf   func(string, ...any)

	readyC chan struct{}
	doneC  chan error
	stderr lastLineWriter
}

func newQuiesceSession(cmd *exec.Cmd, stdinR io.Closer, stdin io.WriteCloser, cancel context.CancelFunc, ready string, logf func(string, ...any)) *quiesceSession {
	return &quiesceSession{
		cmd:    cmd,
		stdinR: stdinR,
		stdin:  stdin,
		cancel: cancel,
		ready:  ready,
		logf:   logf,
		readyC: make(chan struct{}),
		doneC:  make(chan error, 1),
	}
}

// start starts the command, writes input on its stdin, and reads its stdout,
// to its end, so it never blocks writing it.
func (t *quiesceSession) start(input string) error {
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		t.cancel()
		return err
	}
	t.cmd.Stderr = &t.stderr
	err = t.cmd.Start()
	// The command holds its end of the pipe.
	_ = t.stdinR.Close()
	if err != nil {
		_ = t.stdin.Close()
		t.cancel()
		return err
	}
	go func() {
		var once sync.Once
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			t.logf("quiesce: %s", line)
			if t.ready != "" && strings.Contains(line, t.ready) {
				once.Do(func() { close(t.readyC) })
			}
		}
		t.doneC <- t.cmd.Wait()
	}()
	if input != "" {
		go func() {
			_, _ = io.WriteString(t.stdin, strings.TrimSuffix(input, "\n")+"\n")
		}()
	}
	return nil
}

// waitReady returns once the command printed its ready line, and ends it
// when it did not within timeout, or exited first.
func (t *quiesceSession) waitReady(timeout time.Duration) error {
	select {
	case <-t.readyC:
		return nil
	case err := <-t.doneC:
		t.cancel()
		return fmt.Errorf("the command ended before saying %q: %s", t.ready, t.failure(err))
	case <-time.After(timeout):
		_ = t.stdin.Close()
		t.cancel()
		return fmt.Errorf("the command did not say %q within %s", t.ready, timeout)
	}
}

// end writes input on the stdin of the command, closes it, and waits for the
// command to exit, within timeout.
func (t *quiesceSession) end(input string, timeout time.Duration) error {
	defer t.cancel()
	if input != "" {
		go func() {
			_, _ = io.WriteString(t.stdin, strings.TrimSuffix(input, "\n")+"\n")
			_ = t.stdin.Close()
		}()
	} else {
		_ = t.stdin.Close()
	}
	select {
	case err := <-t.doneC:
		if err != nil {
			return fmt.Errorf("the command failed: %s", t.failure(err))
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("the command did not end within %s", timeout)
	}
}

// failure says the error of the command, with the last line of its stderr.
func (t *quiesceSession) failure(err error) string {
	if err == nil {
		err = errors.New("exit status 0")
	}
	if last := t.stderr.Last(); last != "" {
		return fmt.Sprintf("%v: %s", err, last)
	}
	return err.Error()
}

// lastLineWriter keeps the last line written, of a bounded length, to say
// why a command failed without holding all it wrote.
type lastLineWriter struct {
	mu      sync.Mutex
	last    string
	partial []byte
}

const lastLineMax = 4096

func (t *lastLineWriter) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.partial = append(t.partial, b...)
	for {
		i := strings.IndexByte(string(t.partial), '\n')
		if i < 0 {
			break
		}
		if line := strings.TrimSpace(string(t.partial[:i])); line != "" {
			t.last = line
		}
		t.partial = t.partial[i+1:]
	}
	if len(t.partial) > lastLineMax {
		t.partial = t.partial[len(t.partial)-lastLineMax:]
	}
	return len(b), nil
}

// Last returns the last line written, or the line being written.
func (t *lastLineWriter) Last() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if line := strings.TrimSpace(string(t.partial)); line != "" {
		return line
	}
	return t.last
}
