package loop

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/fcache"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/udevadm"
)

const (
	losetup string = "losetup"
)

type (
	T struct {
		log *plog.Logger
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
func WithLogger(log *plog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = log
		return nil
	})
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
	if i == nil {
		return nil
	}
	return t.Delete(i.Name)
}

func (t T) FileGet(filePath string) (*InfoEntry, error) {
	data, err := t.Data()
	if err != nil {
		return nil, err
	}
	return data.File(filePath), nil
}

func (t T) Get(name string) (*InfoEntry, error) {
	data, err := t.Data()
	if err != nil {
		return nil, err
	}
	return data.Name(name), nil
}

func (t T) Data() (InfoEntries, error) {
	var (
		out []byte
		err error
	)
	data := Info{}
	cmd := command.New(
		command.WithName(losetup),
		command.WithVarArgs("-J"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
	)
	if out, err = fcache.Output(cmd, "losetup"); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return InfoEntries{}, nil
	}
	if err = json.Unmarshal(out, &data); err != nil {
		return nil, err
	}
	return data.LoopDevices, nil
}

func (t T) Add(filePath string) error {
	p := "/var/lock/opensvc.losetup.lock"
	lock := flock.New(p, "", fcntllock.New)
	if err := lock.Lock(20*time.Second, ""); err != nil {
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
	fcache.Clear("losetup")
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
	fcache.Clear("losetup")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	udevadm.Settle()
	limit := time.Now().Add(5 * time.Second)
	for {
		info, _ := t.Get(devPath)
		if info == nil {
			return nil
		}
		if time.Now().After(limit) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("losetup silently failed to delete %s", devPath)
}

func (t InfoEntries) Name(s string) *InfoEntry {
	for _, i := range t {
		if i.Name == s {
			return &i
		}
	}
	return nil
}

func (t InfoEntries) File(s string) *InfoEntry {
	for _, i := range t {
		if i.BackFile == s {
			return &i
		}
		if i.BackFile == s+" (deleted)" {
			return &i
		}
	}
	return nil
}

func (t InfoEntries) HasFile(s string) bool {
	return t.File(s) != nil
}
