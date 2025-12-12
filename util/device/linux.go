//go:build linux

package device

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/yookoala/realpath"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/devicedriver"
	"github.com/opensvc/om3/v3/util/udevadm"
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
	if b, err := os.ReadFile(p); err != nil {
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

func (t T) Rescan() error {
	p, err := t.sysfsFile()
	if err != nil {
		return err
	}
	p = p + "/device/rescan"
	return os.WriteFile(p, []byte("1"), os.ModePerm)
}

func (t T) Delete() error {
	p, err := t.sysfsFile()
	if err != nil {
		return err
	}
	p = p + "/device/delete"
	return os.WriteFile(p, []byte("1"), os.ModePerm)
}

func (t T) SlaveHosts() ([]string, error) {
	var errs error
	l := make([]string, 0)
	slaves, err := t.Slaves()
	if err != nil {
		return l, err
	}
	for _, slave := range slaves {
		if host, err := slave.Host(); err != nil {
			errs = errors.Join(errs, err)
			continue
		} else {
			l = append(l, host)
		}
	}
	return l, nil
}

func (t T) Host() (string, error) {
	p, err := t.sysfsFile()
	if err != nil {
		return "", err
	}
	p += "/device"
	devicePath, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	hbtl := strings.Split(filepath.Base(devicePath), ":")
	if len(hbtl) == 4 {
		return "", fmt.Errorf("dev %s host device path unexpected format: %v", devicePath, hbtl)
	}
	return "host" + hbtl[0], nil
}

func (t T) Slaves() (L, error) {
	l := make(L, 0)
	root, err := t.sysfsFile()
	if err != nil {
		return l, err
	}
	root = root + "/slaves"
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

func (t T) Holders() (L, error) {
	l := make(L, 0)
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

func (t T) IsReservable() (bool, error) {
	if v, err := t.IsMultipath(); err != nil {
		return false, err
	} else if v {
		return true, nil
	}
	if v, err := t.IsSCSI(); err != nil {
		return false, err
	} else if v {
		return true, nil
	}
	return false, nil
}

func (t T) IsMultipath() (bool, error) {
	p, err := t.sysfsFile()
	if err != nil {
		return false, err
	}
	p += "/dm/uuid"
	b, err := os.ReadFile(p)
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	}
	s := string(b)
	if strings.HasPrefix(s, "mpath") {
		return true, nil
	}
	return false, nil
}

func (t T) Vendor() (string, error) {
	return t.identityString("vendor")
}

func (t T) Model() (string, error) {
	return t.identityString("model")
}

func (t T) Version() (string, error) {
	return t.identityString("version")
}

func (t T) identityString(s string) (string, error) {
	isMultipath, err := t.IsMultipath()
	if err != nil {
		return "", err
	}
	if isMultipath {
		slaves, err := t.Slaves()
		if err != nil {
			return "", err
		}
		for _, slave := range slaves {
			return slave.scsiIdentityString(s)
		}
		return "", fmt.Errorf("%s has no slave to query for %s", t, s)
	} else {
		return t.scsiIdentityString(s)
	}
}

func (t T) scsiIdentityString(s string) (string, error) {
	p, err := t.sysfsFile()
	if err != nil {
		return "", err
	}
	p += "/device/" + s
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	id := string(b)
	return strings.TrimSpace(id), nil
}

func (t T) IsSCSI() (bool, error) {
	if p, err := t.sysfsFile(); err != nil {
		return false, err
	} else {
		p += "/device/scsi_device"
		_, err := os.Stat(p)
		switch {
		case os.IsNotExist(err):
			return false, nil
		case err != nil:
			return false, err
		default:
			return true, nil
		}
	}
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
		t.log.Tracef("Remove() not implemented for device driver %s", driver)
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

func (t T) ConfigureMultipath(verbosity int) error {
	cmd := command.New(
		command.WithName("multipath"),
		command.WithVarArgs("-v", fmt.Sprint(verbosity), t.path),
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

func (t T) RefreshMultipath() error {
	cmd := command.New(
		command.WithName("multipath"),
		command.WithVarArgs("-r", t.path),
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

func (t T) RemoveMultipath() error {
	cmd := command.New(
		command.WithName("multipath"),
		command.WithVarArgs("-f", t.path),
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
	props, err := udevadm.Properties(t.path)
	if err != nil {
		return "", err
	}
	if v, ok := props["DM_SERIAL"]; ok {
		return v, nil
	}
	if v, ok := props["ID_SERIAL"]; ok {
		return v, nil
	}
	return "", nil
}

func (t T) IsReady() (bool, error) {
	cmd := command.New(
		command.WithName("sg_turs"),
		command.WithVarArgs(t.path),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithIgnoredExitCodes(0, 2),
	)
	err := cmd.Run()
	if cmd.ExitCode() == 2 {
		return false, err
	}
	return true, err
}

func (t T) WaitReady() error {
	delay := time.Second
	retries := 5
	for i := 0; i < retries; i++ {
		if v, err := t.IsReady(); err != nil {
			return err
		} else if v {
			if i == 0 {
				t.log.Infof("waiting for device %s to become ready (max %s)", t.path, time.Duration(retries)*delay)
			}
			time.Sleep(delay)
			continue
		} else {
			return nil
		}
	}
	return fmt.Errorf("timed out waiting for device %s to become ready (max %s)", t.path, time.Duration(retries)*delay)
}

func (t T) PromoteRW() error {
	count := 0
	paths, err := t.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		isChanged := false
		isRO, err := path.IsReadOnly()
		if err != nil {
			return err
		}
		if isRO {
			if err := path.SetReadWrite(); err != nil {
				return err
			}
			isChanged = true
		}
		_, err = path.Stat()
		switch {
		case err == nil:
		case errors.Is(err, os.ErrNotExist):
			if err := t.Rescan(); err != nil {
				return err
			}
			isChanged = true
		default:
			return err
		}
		if isChanged {
			count += 1
			if err := t.WaitReady(); err != nil {
				return err
			}
		}
	}
	isRO, err := t.IsReadOnly()
	if err != nil {
		return err
	}
	if isRO {
		if err := t.SetReadWrite(); err != nil {
			return err
		}
	}
	if count > 0 {
		if err := t.RefreshMultipath(); err != nil {
			return err
		}
	}
	return nil
}
