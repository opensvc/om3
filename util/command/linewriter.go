package command

import (
	"bytes"
	"sync"
)

// lineWriter turns the bytes a command writes into the lines its reader was
// promised, and keeps them if asked to.
//
// It is an io.Writer rather than a goroutine reading a pipe, because that is
// what lets os/exec own the copying: exec.Cmd creates the pipe, copies from it
// in a goroutine of its own, and waits for that goroutine in Wait, bounded by
// WaitDelay. A pipe this package reads itself is a pipe nothing bounds, and a
// command leaving an orphan holding the write end is then waited for for ever.
type lineWriter struct {
	mu      sync.Mutex
	partial []byte
	onLine  func(string)
	collect *[]byte
}

func (t *lineWriter) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.collect != nil {
		*t.collect = append(*t.collect, p...)
	}
	t.partial = append(t.partial, p...)
	for {
		i := bytes.IndexByte(t.partial, '\n')
		if i < 0 {
			break
		}
		t.line(t.partial[:i])
		t.partial = t.partial[i+1:]
	}
	return len(p), nil
}

// Close hands over what the command wrote after its last newline, which is a
// line like any other to whoever is reading them.
func (t *lineWriter) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.partial) > 0 {
		t.line(t.partial)
		t.partial = nil
	}
	return nil
}

func (t *lineWriter) line(b []byte) {
	if t.onLine != nil {
		t.onLine(string(b))
	}
}
