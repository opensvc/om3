package rescontainerkvm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	iofs "io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/fsnotify/fsnotify"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/versioncmp"
)

// serialLogDir is where libvirt keeps the logs of the domains, which virtlogd
// rotates, and root alone reads. A var for the tests.
var serialLogDir = "/var/log/libvirt/qemu"

const (
	// serialLogCap is the capability of a node able to log a serial port.
	serialLogCap = ".serial_log"

	// minSerialLogLibvirt and minSerialLogQemu are the first versions with
	// the log of a character device.
	minSerialLogLibvirt = "2.3.0"
	minSerialLogQemu    = "2.6.0"
)

// maxPartialLine is what a line with no end is held for, before it is sent as
// it is: the guest says what is written here.
const maxPartialLine = 64 * 1024

var (
	serialRe    = regexp.MustCompile(`(?s)<serial\b[^>]*>.*?</serial>`)
	serialPtyRe = regexp.MustCompile(`<serial type='pty'>`)
	serialLogRe = regexp.MustCompile(`<log\b[^>]*\bfile='([^']*)'`)

	virshVersionRe = regexp.MustCompile(`(?s)Using library: libvirt ([0-9.]+).*Running hypervisor: QEMU ([0-9.]+)`)

	// What drives a terminal: the sequences, then the controls left, a tab apart.
	escapeRe  = regexp.MustCompile(`\x1b(\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)?|[ -/]*[0-~])?`)
	controlRe = regexp.MustCompile(`[\x00-\x08\x0a-\x1f\x7f\x{80}-\x{9f}]`)
)

// isSerialLogCapable says the node can log a serial port: libvirt refuses a
// definition logging one on a qemu that can not, and the guest does not start.
func isSerialLogCapable(ctx context.Context) bool {
	b, err := exec.CommandContext(ctx, "virsh", "version").Output()
	return err == nil && canLogSerial(b)
}

// canLogSerial says what virsh version tells of the versions of libvirt and qemu
// is able to log a serial port.
func canLogSerial(b []byte) bool {
	m := virshVersionRe.FindSubmatch(b)
	if m == nil {
		return false
	}
	libvirt, err := versioncmp.AtLeast(string(m[1]), minSerialLogLibvirt)
	if err != nil || !libvirt {
		return false
	}
	qemu, err := versioncmp.AtLeast(string(m[2]), minSerialLogQemu)
	return err == nil && qemu
}

// canLogSerial says this node can log the serial port of a guest.
func (t *T) canLogSerial() bool {
	return capabilities.Has(drvID.Cap() + serialLogCap)
}

// doSerialLog writes the log of the serial port in the definition file of the
// domain, or removes the one a peer able to log it wrote there.
func (t *T) doSerialLog() error {
	if t.canLogSerial() {
		return setSerialLog(t.configFile(), t.serialLogPath())
	}
	return unsetSerialLog(t.configFile(), t.serialLogPath())
}

// serialLogPath is the file a guest writes its serial port to. The qemu log of
// the domain is the one of the hypervisor, next to it.
func (t *T) serialLogPath() string {
	return filepath.Join(serialLogDir, t.Name+".serial.log")
}

// removeSerialLog removes what a guest going away said, for the next guest of
// this name not to be read as saying it.
func (t *T) removeSerialLog() error {
	if err := os.Remove(t.serialLogPath()); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return nil
}

// definedSerialLog returns the file a definition logs a serial port to, empty
// when it logs none.
func definedSerialLog(b []byte) string {
	for _, serial := range serialRe.FindAll(b, -1) {
		if m := serialLogRe.FindSubmatch(serial); m != nil {
			return html.UnescapeString(string(m[1]))
		}
	}
	return ""
}

// definitionWithSerialLog returns the definition of a domain with its first
// pty serial port logged to this file, the rest of it left as libvirt wrote
// it. A definition logging a port already says where.
func definitionWithSerialLog(b []byte, file string) []byte {
	if definedSerialLog(b) != "" {
		return b
	}
	loc := serialPtyRe.FindIndex(b)
	if loc == nil {
		return b
	}
	tag := "\n      <log file='" + html.EscapeString(file) + "' append='on'/>"
	after := make([]byte, 0, len(b)+len(tag))
	after = append(after, b[:loc[1]]...)
	after = append(after, tag...)
	return append(after, b[loc[1]:]...)
}

// definitionWithoutSerialLog returns the definition of a domain with this file
// no longer logging a serial port. Any other file is a sysadmin's.
func definitionWithoutSerialLog(b []byte, file string) []byte {
	tag := "\n      <log file='" + html.EscapeString(file) + "' append='on'/>"
	return bytes.ReplaceAll(b, []byte(tag), nil)
}

// ContainerLogs reads what the guest writes to its serial port, the output of
// a process for a virtual machine: what the kernel, the init system and
// cloud-init say. Followed, a log still to come is waited for, a guest
// writing it from its next start on.
func (t *T) ContainerLogs(ctx context.Context, follow bool, lines int) (<-chan []byte, error) {
	b, err := os.ReadFile(t.configFile())
	if err != nil {
		return nil, err
	}
	file := definedSerialLog(b)
	if file == "" {
		return nil, fmt.Errorf("%s logs no serial port", t.Name)
	}
	if !follow {
		if _, err := os.Stat(file); errors.Is(err, iofs.ErrNotExist) {
			return nil, fmt.Errorf("%s has not started on this node since it logs its serial port", t.Name)
		}
	}
	return tailLog(ctx, file, follow, lines)
}

// tailLog reads the last lines of a file and, followed, those it gets next,
// across the rotation that renames it and the one that truncates it.
func tailLog(ctx context.Context, file string, follow bool, lines int) (<-chan []byte, error) {
	var (
		events       chan fsnotify.Event
		watchErrors  chan error
		closeWatcher func()
	)
	if follow {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		if err := w.Add(filepath.Dir(file)); err != nil {
			_ = w.Close()
			return nil, err
		}
		events, watchErrors = w.Events, w.Errors
		closeWatcher = func() { _ = w.Close() }
	}
	ch := make(chan []byte)
	go func() {
		defer close(ch)
		if closeWatcher != nil {
			defer closeWatcher()
		}
		r := &logReader{file: file, ch: ch}
		defer r.close()
		// Lines are last ones once: the file a rotation brings is read whole.
		first := lines
		for {
			if err := r.read(ctx, first); err != nil {
				return
			}
			first = -1
			if !follow {
				_ = r.flush(ctx)
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case ev, ok := <-events:
					if !ok {
						return
					}
					if ev.Name != file {
						continue
					}
				case err, ok := <-watchErrors:
					// Events were lost: all there is to do is to read.
					if !ok || err == nil {
						return
					}
				}
				break
			}
		}
	}()
	return ch, nil
}

// logReader is where a tailLog is in its file.
type logReader struct {
	file   string
	ch     chan<- []byte
	f      *os.File
	offset int64
	line   []byte
}

func (r *logReader) close() {
	if r.f != nil {
		_ = r.f.Close()
		r.f = nil
	}
}

// read sends what the file got since the last read, its last lines at the
// first one, unless there are no such lines to skip to (a negative number).
func (r *logReader) read(ctx context.Context, lines int) error {
	if r.f != nil {
		if err := r.readNew(ctx, -1); err != nil {
			return err
		}
		// What is written from now on goes to another file, or to the start.
		now, err := os.Stat(r.file)
		held, heldErr := r.f.Stat()
		switch {
		case err != nil || heldErr != nil || !os.SameFile(now, held):
			r.close()
		case now.Size() < r.offset:
			r.offset = 0
			return r.readNew(ctx, -1)
		default:
			return nil
		}
	}
	f, err := os.Open(r.file)
	if errors.Is(err, iofs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	r.f = f
	r.offset = 0
	return r.readNew(ctx, lines)
}

func (r *logReader) readNew(ctx context.Context, lines int) error {
	if _, err := r.f.Seek(r.offset, io.SeekStart); err != nil {
		return err
	}
	b, err := io.ReadAll(r.f)
	if err != nil {
		return err
	}
	r.offset += int64(len(b))
	if lines >= 0 {
		b = lastLines(b, lines)
	}
	return r.send(ctx, b)
}

// send cuts b in lines, the last one kept until it ends.
func (r *logReader) send(ctx context.Context, b []byte) error {
	r.line = append(r.line, b...)
	for {
		i := bytes.IndexByte(r.line, '\n')
		if i < 0 {
			break
		}
		if err := r.sendLine(ctx, r.line[:i]); err != nil {
			return err
		}
		r.line = r.line[i+1:]
	}
	if len(r.line) > maxPartialLine {
		return r.flush(ctx)
	}
	return nil
}

func (r *logReader) flush(ctx context.Context) error {
	if len(r.line) == 0 {
		return nil
	}
	err := r.sendLine(ctx, r.line)
	r.line = nil
	return err
}

func (r *logReader) sendLine(ctx context.Context, b []byte) error {
	select {
	case r.ch <- append(cleanLine(b), '\n'):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// cleanLine returns the text of a line of a serial port, which is what a
// terminal is sent: the guest is not to drive the terminal of who reads it.
func cleanLine(b []byte) []byte {
	b = bytes.ToValidUTF8(b, nil)
	b = escapeRe.ReplaceAll(b, nil)
	return controlRe.ReplaceAll(b, nil)
}

// lastLines returns the end of b from its nth line before the last.
func lastLines(b []byte, n int) []byte {
	if n == 0 {
		return nil
	}
	end := len(b)
	if end > 0 && b[end-1] == '\n' {
		end--
	}
	for ; n > 0; n-- {
		i := bytes.LastIndexByte(b[:end], '\n')
		if i < 0 {
			return b
		}
		if n > 1 {
			end = i
			continue
		}
		return b[i+1:]
	}
	return b
}
