package evwatch

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// streamWaitDelay bounds the wait for a command stopped: its output pipe is
// closed then, whatever holds it open, as a child it left running.
const streamWaitDelay = 3 * time.Second

// stderrTailLines is how many of the last lines a command writes to its
// stderr are kept, to say why it ended.
const stderrTailLines = 5

// StreamLines runs name with args, the variables of env added to the
// environment, and calls onLine for each line it prints, until it ends or
// the context is done. The command is then sent a SIGTERM, and its output
// closed streamWaitDelay later, so a stop never waits on it longer.
//
// It returns the last lines of the stderr of the command, to say why it
// ended, and its error.
func StreamLines(ctx context.Context, env []string, onLine func(string), name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = streamWaitDelay
	tail := &lineTail{max: stderrTailLines}
	cmd.Stderr = tail
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		onLine(scanner.Text())
	}
	errScan := scanner.Err()
	if errors.Is(errScan, os.ErrClosed) {
		// The pipe closed by the wait delay.
		errScan = nil
	}
	errWait := cmd.Wait()
	return tail.String(), errors.Join(errScan, errWait)
}

// lineTail keeps the last lines written to it.
type lineTail struct {
	mu    sync.Mutex
	max   int
	lines []string
	part  string
}

func (t *lineTail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.part + string(b)
	l := strings.Split(s, "\n")
	t.part = l[len(l)-1]
	for _, line := range l[:len(l)-1] {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		t.lines = append(t.lines, line)
		if len(t.lines) > t.max {
			t.lines = t.lines[1:]
		}
	}
	return len(b), nil
}

func (t *lineTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	l := t.lines
	if part := strings.TrimSpace(t.part); part != "" {
		l = append(l, part)
	}
	return strings.Join(l, "; ")
}
