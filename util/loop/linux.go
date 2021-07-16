// +build linux

package loop

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
)

const (
	losetup string = "losetup"
)

type (
	T struct {
		log *zerolog.Logger
	}
	Info struct {
		LoopDevices []InfoEntry `json:"loopdevices"`
	}
	InfoEntry struct {
		Name      string `json:"name"`      // "/dev/loop1"
		SizeLimit int64  `json:"sizelimit"` // 0
		Offset    int64  `json:"offset"`    // 0
		AutoClear bool   `json:"autoclear"` // true
		ReadOnly  bool   `json:"ro"`        // true
		BackFile  string `json:"back-file"` // "/var/lib/snapd/snaps/gnome-3-34-1804_66.snap"
		DirectIO  bool   `json:"dio"`       // false
		LogSec    int64  `json:"log-sec"`   // 512
	}
	InfoEntries []InfoEntry
)

func New(opts ...funcopt.O) *T {
	t := T{}
	_ = funcopt.Apply(&t, opts...)
	return &t
}
func WithLogger(log *zerolog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

func IsCapable() bool {
	if _, err := exec.LookPath(losetup); err != nil {
		return false
	}
	return true
}

func (t T) FileExists(filePath string) (bool, error) {
	data, err := t.Data()
	if err != nil {
		return false, err
	}
	return data.HasFile(filePath), nil
}

func (t T) FileDelete(filePath string) error {
	i, err := t.FileGet(filePath)
	if err != nil {
		return err
	}
	return t.Delete(i.Name)
}

func (t T) FileGet(filePath string) (*InfoEntry, error) {
	data, err := t.Data()
	if err != nil {
		return nil, err
	}
	e := data.File(filePath)
	if e == nil {
		return nil, fmt.Errorf("no loop info for %s", filePath)
	}
	return e, nil
}

func (t T) Data() (InfoEntries, error) {
	data := Info{}
	cmd := command.New(
		command.WithName(losetup),
		command.WithVarArgs("-J"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cmd.Stdout(), &data); err != nil {
		return nil, err
	}
	return InfoEntries(data.LoopDevices), nil
}

func (t T) Add(filePath string) error {
	p := "/var/lock/opensvc.losetup.lock"
	lock := flock.New(p, "", fcntllock.New)
	timeout, err := time.ParseDuration("20s")
	if err != nil {
		return err
	}
	err = lock.Lock(timeout, "")
	if err != nil {
		return err
	}
	defer func() { _ = lock.UnLock() }()
	return t.lockedAdd(filePath)
}

func (t T) lockedAdd(filePath string) error {
	cmd := command.New(
		command.WithName(losetup),
		command.WithVarArgs("-f", filePath),
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

func (t T) Delete(devPath string) error {
	cmd := command.New(
		command.WithName(losetup),
		command.WithVarArgs("-d", devPath),
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

func (t InfoEntries) File(s string) *InfoEntry {
	for _, i := range t {
		if i.BackFile == s {
			return &i
		}
	}
	return nil
}

func (t InfoEntries) HasFile(s string) bool {
	return t.File(s) != nil
}
