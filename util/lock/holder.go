package lock

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opensvc/flock"

	"github.com/opensvc/om3/v3/util/args"
)

// timeoutMessage is the error of flock when the lock is still held at the
// timeout, which it makes no sentinel of.
const timeoutMessage = "lock timeout exceeded"

// Explain returns err, the failure to take lock within timeout, with what
// holds the lock: the pid of the holder and its command line while it runs,
// its intent, its session and since when, as the holder wrote them in the
// lock file.
//
// A lock released in the meantime, or taken by a holder that has not
// written who it is yet, is told as such.
func Explain(lock *flock.T, timeout time.Duration, err error) error {
	if err == nil || err.Error() != timeoutMessage {
		// Not a timeout, as a lock file the process can not open: the
		// holder is not the reason.
		return err
	}
	meta, probeErr := lock.Probe()
	switch {
	case probeErr != nil:
		return fmt.Errorf("%w after %s on %s, by a holder not known yet", err, timeout, lock.Path)
	case meta.PID == 0:
		return fmt.Errorf("%w after %s on %s, released since", err, timeout, lock.Path)
	default:
		return fmt.Errorf("%w after %s on %s, held by %s", err, timeout, lock.Path, describeHolder(meta, time.Now()))
	}
}

// describeHolder tells the holder of a lock, as
// "pid 1234 (/usr/bin/om svc1 instance status -r) for status since
// 2026-10-08T07:45:07Z (1s ago), session 7bcd2c78-...".
func describeHolder(meta flock.Meta, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pid %d", meta.PID)
	if cmdline := commandLine(meta.PID); cmdline != "" {
		fmt.Fprintf(&b, " (%s)", cmdline)
	}
	if meta.Intent != "" {
		fmt.Fprintf(&b, " for %s", meta.Intent)
	}
	if !meta.At.IsZero() {
		fmt.Fprintf(&b, " since %s (%s ago)", meta.At.Format(time.RFC3339), now.Sub(meta.At).Round(time.Millisecond))
	}
	if meta.SessionID != "" {
		fmt.Fprintf(&b, ", session %s", meta.SessionID)
	}
	return b.String()
}

// commandLine returns the command line of the process, the values of the
// flags that may hold secrets masked, as the action logs mask them, and
// empty when the process does not run any more or the system does not
// tell it.
func commandLine(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	b = bytes.TrimRight(b, "\x00")
	if len(b) == 0 {
		return ""
	}
	argv := strings.Split(string(b), "\x00")
	return strings.TrimSpace(strings.Join(args.MaskSecrets(argv), " "))
}
