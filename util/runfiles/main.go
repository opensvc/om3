package runfiles

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/bootid"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/proc"
)

type (
	Dir struct {
		Path string
		Log  *plog.Logger
	}
	List []Info
	Info struct {
		At      time.Time
		PID     int
		Content []byte
	}
	cleaning int
)

var (
	ErrProcNotFound = errors.New("process not found")
	ErrProcTooYoung = errors.New("process too young")
)

const (
	doClean cleaning = iota
	noClean
)

// headerPrefix begins the first line of a run file, naming the process that
// wrote it: "run <start time> <boot id>", the start time in clock ticks since
// the boot. A pid is reused, but not with the same start time before a
// reboot, so the line tells the writer from a process given its pid since.
const headerPrefix = "run "

// header returns the identity line of the process pid, nil when it can not
// be read, the file then written as before, judged by its time.
func header(pid int) []byte {
	start, err := proc.New(pid).StartTime()
	if err != nil {
		return nil
	}
	boot := bootid.Get()
	if boot == "" {
		return nil
	}
	return []byte(fmt.Sprintf("%s%d %s\n", headerPrefix, start, boot))
}

// parse returns the identity of the writer of the run file b, ok false for a
// file written before the identity line, and the content after the line.
func parse(b []byte) (start uint64, boot string, content []byte, ok bool) {
	if !bytes.HasPrefix(b, []byte(headerPrefix)) {
		return 0, "", b, false
	}
	line, rest, found := bytes.Cut(b, []byte("\n"))
	if !found {
		return 0, "", b, false
	}
	fields := strings.Fields(strings.TrimPrefix(string(line), headerPrefix))
	if len(fields) != 2 {
		return 0, "", b, false
	}
	start, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, "", b, false
	}
	return start, fields[1], rest, true
}

func (t Dir) filename(pid int) string {
	return filepath.Join(t.Path, fmt.Sprint(pid))
}

func (t Dir) Remove() error {
	return t.remove(os.Getpid())
}

func (t Dir) Create(content []byte) error {
	return t.create(os.Getpid(), content)
}

func (t Dir) remove(pid int) error {
	filename := t.filename(pid)
	return os.Remove(filename)
}

func (t Dir) create(pid int, content []byte) error {
	filename := t.filename(pid)
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	switch {
	case os.IsNotExist(err):
		if err := os.MkdirAll(t.Path, os.ModePerm); err != nil {
			return err
		}
		file, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
		if err != nil {
			return err
		}
	case err != nil:
		return err
	}
	defer file.Close()
	defer file.Sync()
	_, err = file.Write(append(header(pid), content...))
	return err
}

func (t Dir) List() (l List, err error) {
	var v bool
	err = filepath.WalkDir(t.Path, func(path string, e os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		if path == t.Path {
			return nil
		}
		if e.IsDir() {
			return filepath.SkipDir
		}
		v, err = IsValid(path)
		if errors.Is(err, ErrProcNotFound) || errors.Is(err, ErrProcTooYoung) {
			return nil
		} else if err != nil {
			return err
		}
		if v {
			pid, err := strconv.Atoi(filepath.Base(path))
			if err != nil {
				return nil
			}
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			_, _, content, _ := parse(b)
			l = append(l, Info{
				PID:     pid,
				Content: content,
				At:      info.ModTime(),
			})
		}
		return nil
	})
	return
}

func (t Dir) HasRunning() (bool, error) {
	var v bool
	err := filepath.WalkDir(t.Path, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == t.Path {
			return nil
		}
		if e.IsDir() {
			return filepath.SkipDir
		}
		v, err = IsValid(path)
		if err != nil {
			return err
		}
		if v {
			return filepath.SkipAll
		}
		return nil
	})
	if os.IsNotExist(err) {
		return v, nil
	}
	return v, err
}

func IsValid(file string) (bool, error) {
	info, err := os.Lstat(file)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if !info.Mode().IsRegular() {
		return false, nil
	}
	pid := filepath.Base(file)
	b, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if start, boot, _, ok := parse(b); ok {
		// The writer is named: the run is the one of the process running
		// with its pid and start time, in this boot.
		n, err := strconv.Atoi(pid)
		if err != nil {
			return false, nil
		}
		if boot != bootid.Get() || !proc.New(n).RunsSince(start) {
			return false, fmt.Errorf("%w: %s", ErrProcNotFound, pid)
		}
		return true, nil
	}
	// A run file of an older agent names no writer: judged by the time of
	// the process directory, which the kernel may recreate, but for the run
	// file of this process, which is its own whatever the time says. A file
	// naming its writer is judged by it, this process included: a stale
	// file of a crashed run may carry the pid this process was given.
	if pid == strconv.Itoa(os.Getpid()) {
		return true, nil
	}
	procFile := "/proc/" + pid
	procInfo, err := os.Lstat(procFile)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("%w: %s", ErrProcNotFound, pid)
	} else if err != nil {
		return false, err
	}
	if info.ModTime().Before(procInfo.ModTime()) {
		return false, fmt.Errorf("%w: %s created %s after run file", ErrProcTooYoung, pid, procInfo.ModTime().Sub(info.ModTime()))
	}
	return true, nil
}

func (t Dir) count(clean cleaning) (int, error) {
	var n int
	err := filepath.WalkDir(t.Path, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == t.Path {
			return nil
		}
		if e.IsDir() {
			return filepath.SkipDir
		}

		v, err := IsValid(path)
		if err != nil && !errors.Is(err, ErrProcNotFound) && !errors.Is(err, ErrProcTooYoung) {
			return err
		}

		if v {
			n += 1
		} else if clean == doClean {
			removeErr := os.Remove(path)
			switch {
			case os.IsNotExist(removeErr):
				return nil
			case removeErr != nil:
				return removeErr
			}
			if t.Log != nil {
				if errors.Is(err, ErrProcNotFound) || errors.Is(err, ErrProcTooYoung) {
					t.Log.Infof("clean up stale run file (%s)", err)
				}
			}
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return n, err
}

func (t Dir) CountAndClean() (int, error) {
	return t.count(doClean)
}

func (t Dir) Count() (int, error) {
	return t.count(noClean)
}
