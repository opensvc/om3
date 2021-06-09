// +build linux

package device

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yookoala/realpath"
	"opensvc.com/opensvc/util/file"
)

func (t T) IsReadWrite() (bool, error) {
	if ro, err := t.IsReadOnly(); err != nil {
		return false, err
	} else {
		return !ro, nil
	}
}

func (t T) IsReadOnly() (bool, error) {
	p, err := t.sysfsFileRO()
	if err != nil {
		return false, err
	}
	if b, err := file.ReadAll(p); err != nil {
		return false, err
	} else {
		return strings.TrimSpace(string(b)) == "1", nil
	}
}

func (t T) SetReadWrite() error {
	return t.setRO(false)
}

func (t T) SetReadOnly() error {
	return t.setRO(true)
}

func (t T) sysfsFileRO() (string, error) {
	canon, err := realpath.Realpath(string(t))
	if err != nil {
		return "", err
	}
	canon = filepath.Base(canon)
	return fmt.Sprintf("/sys/block/%s/ro", canon), nil
}

func (t T) setRO(v bool) error {
	var action string
	if v {
		action = "--setro"
	} else {
		action = "--setrw"
	}
	cmd := exec.Command("blockdev", action, string(t))
	cmd.Start()
	if err := cmd.Wait(); err != nil {
		return err
	}
	exitCode := cmd.ProcessState.ExitCode()
	if exitCode != 0 {
		cmdStr := fmt.Sprintf("blockdev %s %s", action, t)
		return fmt.Errorf("%s returned %d", cmdStr, exitCode)
	}
	return nil
}
