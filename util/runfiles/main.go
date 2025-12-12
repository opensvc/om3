package runfiles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/opensvc/om3/v3/util/plog"
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
	_, err = file.Write(content)
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
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
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
