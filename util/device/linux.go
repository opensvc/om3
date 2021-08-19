// +build linux

package device

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/yookoala/realpath"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/devicedriver"
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

func (t T) sysfsFile() (string, error) {
	canon, err := realpath.Realpath(t.path)
	if err != nil {
		return "", err
	}
	canon = filepath.Base(canon)
	return fmt.Sprintf("/sys/block/%s", canon), nil
}

func (t T) sysfsFileRO() (string, error) {
	p, err := t.sysfsFile()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/ro", p), nil
}

func (t T) setRO(v bool) error {
	var action string
	if v {
		action = "--setro"
	} else {
		action = "--setrw"
	}
	cmd := command.New(
		command.WithName("blockdev"),
		command.WithVarArgs(action, t.path),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s returned %d", cmd, exitCode)
	}
	return nil
}

func (t T) Holders() ([]*T, error) {
	l := make([]*T, 0)
	root, err := t.sysfsFile()
	if err != nil {
		return l, err
	}
	root = root + "/holders"
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		dev := New("/dev/"+filepath.Base(path), WithLogger(t.log))
		l = append(l, dev)
		return nil
	})
	return l, nil
}

func (t T) Driver() (interface{}, error) {
	major, err := t.Major()
	if err != nil {
		return nil, err
	}
	return devicedriver.NewFromMajor(major, devicedriver.WithLogger(t.log)), nil
}

func (t T) Remove() error {
	driver, err := t.Driver()
	if err != nil {
		return err
	}
	type remover interface {
		Remove(T) error
	}
	driverRemover, ok := driver.(remover)
	if !ok {
		t.log.Debug().Msgf("Remove() not implemented for device driver %s", driver)
		return nil
	}
	driverRemover.Remove(t)
	return nil
}

func (t T) Wipe() error {
	cmd := command.New(
		command.WithName("wipefs"),
		command.WithVarArgs("-a", t.path),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t T) WWID() (string, error) {
	return "", nil
}
