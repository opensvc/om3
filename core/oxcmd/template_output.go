package oxcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

type (
	// templateOutput writes a rendered template to a file, as a whole: a
	// reader of the file sees the former content or the new, never a part.
	templateOutput struct {
		path string

		// mode is the mode of the file, nil to keep the mode of the
		// file it replaces, or 0600 for a new one.
		mode *fs.FileMode

		// uid and gid are the owner of the file, -1 to keep the owner
		// of the file it replaces, or the user writing a new one.
		uid, gid int
	}

	// reloadError is the error of the reload command, run after the output
	// changed.
	reloadError struct {
		err error
	}
)

// defaultTemplateOutputMode is the mode of a new output: a rendered template
// may hold secrets.
const defaultTemplateOutputMode fs.FileMode = 0o600

func (t *reloadError) Error() string {
	return "reload: " + t.err.Error()
}

func (t *reloadError) Unwrap() error {
	return t.err
}

// ExitCode tells a failed reload from a failed render.
func (t *reloadError) ExitCode() int {
	return 3
}

// newTemplateOutput returns the output path, with the mode, octal, and the
// owner user and group, by name or id, empty to keep the ones of the file.
func newTemplateOutput(path, mode, usr, grp string) (*templateOutput, error) {
	t := &templateOutput{path: path, uid: -1, gid: -1}
	if mode != "" {
		i, err := strconv.ParseUint(mode, 8, 32)
		if err != nil || i > 0o7777 {
			return nil, fmt.Errorf("invalid mode %s: an octal mode is expected, as 0640", mode)
		}
		m := fs.FileMode(i)
		t.mode = &m
	}
	if usr != "" {
		uid, err := lookupID(usr, func(s string) (string, error) {
			u, err := user.Lookup(s)
			if err != nil {
				return "", err
			}
			return u.Uid, nil
		})
		if err != nil {
			return nil, fmt.Errorf("user %s: %w", usr, err)
		}
		t.uid = uid
	}
	if grp != "" {
		gid, err := lookupID(grp, func(s string) (string, error) {
			g, err := user.LookupGroup(s)
			if err != nil {
				return "", err
			}
			return g.Gid, nil
		})
		if err != nil {
			return nil, fmt.Errorf("group %s: %w", grp, err)
		}
		t.gid = gid
	}
	return t, nil
}

func lookupID(s string, lookup func(string) (string, error)) (int, error) {
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	}
	id, err := lookup(s)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(id)
}

// isGroupOrWorldReadable returns true when the mode set for the output lets
// others than its owner read it.
func (t *templateOutput) isGroupOrWorldReadable() bool {
	return t.mode != nil && *t.mode&0o044 != 0
}

// check returns an error when the output is a symbolic link: the file it
// points to may not be the one meant, and would be replaced by a file.
func (t *templateOutput) check() (fs.FileInfo, error) {
	fi, err := os.Lstat(t.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is a symbolic link: refused, name the file it points to", t.path)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", t.path)
	}
	return fi, nil
}

// write writes b to the output, unless it holds b already, and returns true
// when it wrote.
func (t *templateOutput) write(b []byte) (bool, error) {
	fi, err := t.check()
	if err != nil {
		return false, err
	}
	mode := defaultTemplateOutputMode
	switch {
	case t.mode != nil:
		mode = *t.mode
	case fi != nil:
		mode = fi.Mode().Perm()
	}
	if fi != nil {
		if same, err := t.hardenIfSame(b); err != nil || same {
			return false, err
		}
	}
	uid, gid := t.uid, t.gid
	if fi != nil {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			if uid < 0 {
				uid = int(st.Uid)
			}
			if gid < 0 {
				gid = int(st.Gid)
			}
		}
	}
	dir, base := filepath.Split(t.path)
	if dir == "" {
		dir = "."
	}
	f, err := os.CreateTemp(dir, "."+base+".tmp*")
	if err != nil {
		return false, err
	}
	tmp := f.Name()
	done := false
	defer func() {
		if !done {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	// The mode and the owner are set before the content is written, so
	// a secret is never readable by others, even for an instant.
	if err := f.Chmod(mode); err != nil {
		return false, err
	}
	if uid >= 0 || gid >= 0 {
		if err := f.Chown(uid, gid); err != nil {
			return false, err
		}
	}
	if _, err := f.Write(b); err != nil {
		return false, err
	}
	if err := f.Sync(); err != nil {
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, t.path); err != nil {
		return false, err
	}
	done = true
	return true, nil
}

// hardenIfSame returns true when the existing output holds b already, after
// setting on it the mode and the owner asked, when they differ: a file holding
// secrets asked 0600 is not left readable by others.
//
// The file is opened not following a symbolic link, and read and changed
// through this open file: a path swapped for a link after the check is never
// followed to another file.
func (t *templateOutput) hardenIfSame(b []byte) (bool, error) {
	f, err := os.OpenFile(t.path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if errors.Is(err, syscall.ELOOP) {
		return false, fmt.Errorf("%s is a symbolic link: refused, name the file it points to", t.path)
	} else if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !fi.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file", t.path)
	}
	current, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(current, b) {
		return false, nil
	}
	if t.mode != nil && fi.Mode().Perm() != *t.mode {
		if err := f.Chmod(*t.mode); err != nil {
			return true, err
		}
	}
	if t.uid < 0 && t.gid < 0 {
		return true, nil
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if (t.uid < 0 || int(st.Uid) == t.uid) && (t.gid < 0 || int(st.Gid) == t.gid) {
			return true, nil
		}
	}
	return true, f.Chown(t.uid, t.gid)
}

// checkReloadCommand returns an error when the reload command p is not an
// executable file.
func checkReloadCommand(p string) error {
	if p == "" {
		return nil
	}
	fi, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("reload command: %w", err)
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("reload command %s is not an executable file", p)
	}
	return nil
}

// runReload runs the reload command p, with no argument and no shell, in a
// process group of its own, killed with its children past timeout. The
// output file is exported as OX_TEMPLATE_OUT.
func runReload(ctx context.Context, p, out string, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, p)
	cmd.Env = append(os.Environ(), "OX_TEMPLATE_OUT="+out)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return &reloadError{err: fmt.Errorf("%s: killed after %s", p, timeout)}
		}
		return &reloadError{err: fmt.Errorf("%s: %w", p, err)}
	}
	return nil
}
