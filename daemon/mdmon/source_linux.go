//go:build linux

package mdmon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// source is the md event source: the kernel signals each md event on
	// /proc/mdstat to the processes polling it.
	source struct {
		log *plog.Logger

		// path is the file signaling the md events, /proc/mdstat.
		path string

		// wait waits up to timeout for the file of fd to signal an event.
		wait func(fd int, timeout time.Duration) (bool, error)

		// read reads the file of fd from its start, which has the kernel
		// take the events signaled so far as seen.
		read func(fd int) ([]byte, error)
	}
)

var (
	// procMdstat is the file the kernel signals the md events on.
	procMdstat = "/proc/mdstat"

	// moduleCheckInterval is how often the source checks for the md
	// module, loaded by the first array created on the node.
	moduleCheckInterval = 5 * time.Second

	// pollTimeout bounds each wait for an event, for the source to see
	// its context end, as mntmon does.
	pollTimeout = 300 * time.Millisecond

	// aliveCheckInterval is how often, with no event, the source checks
	// the file is still there: an md module unloaded removes it.
	aliveCheckInterval = 10 * time.Second

	// eventSettle is how long after an event the file is read: the kernel
	// signals a transition of an array more than once, as the start of a
	// check, and the events signaled meanwhile are seen by the one read.
	// Read at once, the second event finds no change, which names every
	// array.
	eventSettle = 250 * time.Millisecond

	// maxEventsPerSecond is the most events a second the source takes
	// before it ends and lets the runner start it again after a delay:
	// a module unloading signals for good.
	maxEventsPerSecond = 50
)

func newSource(log *plog.Logger) *source {
	return &source{log: log, path: procMdstat, wait: pollPri, read: readAll}
}

// Name implements evwatch.Source.
func (t *source) Name() string {
	return "md events"
}

// Check implements evwatch.Source: nothing to install, the events are the
// kernel's. A module not loaded yet is waited for by Run.
func (t *source) Check(context.Context) error {
	return nil
}

// Run implements evwatch.Source: it waits for the md events signaled on the
// file, ready once the file is read, and emits the kernel name of each array
// the event changed, or "" when it tells none, as an array resynced whole
// between two reads.
//
// The kernel signals an event to a poller of /proc/mdstat until the poller
// reads the file again from its start: the file is read again on the polled
// descriptor after each event, as "mdadm --monitor" does, or the next poll
// would return at once, for ever. That read is the content the arrays are
// told apart by, so a change after it signals a new event.
func (t *source) Run(ctx context.Context, ready func(), emit func(string)) error {
	if !t.waitModule(ctx) {
		return ctx.Err()
	}
	fd, err := unix.Open(t.path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", t.path, err)
	}
	defer func() { _ = unix.Close(fd) }()
	b, err := t.read(fd)
	if err != nil {
		return fmt.Errorf("read %s: %w", t.path, err)
	}
	arrays := parseMdstat(b)
	t.log.Infof("watch the md events of %s", t.path)
	ready()
	lastAliveCheck := time.Now()
	windowStart := time.Now()
	windowEvents := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		fired, err := t.wait(fd, pollTimeout)
		if err != nil {
			return fmt.Errorf("poll %s: %w", t.path, err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !fired {
			if time.Since(lastAliveCheck) >= aliveCheckInterval {
				lastAliveCheck = time.Now()
				if _, err := os.Stat(t.path); err != nil {
					return fmt.Errorf("the md module is unloaded: %w", err)
				}
			}
			continue
		}
		if time.Since(windowStart) >= time.Second {
			windowStart = time.Now()
			windowEvents = 0
		}
		windowEvents++
		if windowEvents > maxEventsPerSecond {
			return fmt.Errorf("%s signals more than %d events a second", t.path, maxEventsPerSecond)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(eventSettle):
		}
		b, err := t.read(fd)
		if err != nil {
			return fmt.Errorf("read %s: %w", t.path, err)
		}
		cur := parseMdstat(b)
		changed := changedArrays(arrays, cur)
		arrays = cur
		if len(changed) == 0 {
			emit("")
			continue
		}
		for _, name := range changed {
			emit(name)
		}
	}
}

// waitModule waits for the md module to be loaded, which the first array
// created on the node does. It returns false when the context is done first.
func (t *source) waitModule(ctx context.Context) bool {
	logged := false
	for {
		if _, err := os.Stat(t.path); err == nil {
			return true
		}
		if !logged {
			t.log.Infof("the md kernel module is not loaded: wait for it")
			logged = true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(moduleCheckInterval):
		}
	}
}

// readAll reads the file of fd from its start to its end, which has the
// kernel take the events signaled so far as seen, and returns its content.
func readAll(fd int) ([]byte, error) {
	if _, err := unix.Seek(fd, 0, io.SeekStart); err != nil {
		return nil, err
	}
	var content []byte
	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(fd, buf)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return content, nil
		}
		content = append(content, buf[:n]...)
	}
}

// pollPri waits up to timeout for the file of fd to signal an event, a
// priority or an error condition: /proc/mdstat is always readable, so POLLIN
// tells nothing.
func pollPri(fd int, timeout time.Duration) (bool, error) {
	pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLPRI}}
	n, err := unix.Poll(pfd, int(timeout.Milliseconds()))
	if errors.Is(err, unix.EINTR) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	return pfd[0].Revents&(unix.POLLPRI|unix.POLLERR) != 0, nil
}
