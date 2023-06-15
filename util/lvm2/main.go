//go:build linux

package lvm2

import (
	"errors"
	"os/exec"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	driver struct {
		log *zerolog.Logger
	}
	ShowData struct {
		Report []LVReport `json:"report"`
	}
	LVReport struct {
		LV []LVInfo `json:"lv"`
		VG []VGInfo `json:"vg"`
	}
)

var (
	ErrExist = errors.New("does not exist")
)

func (t driver) DriverName() string {
	return "lvm2"
}

func (t *driver) SetLog(log *zerolog.Logger) {
	t.log = log
}

func (t *driver) Log() *zerolog.Logger {
	return t.log
}

func WithLogger(log *zerolog.Logger) funcopt.O {
	type setLoger interface {
		SetLog(*zerolog.Logger)
	}
	return funcopt.F(func(i interface{}) error {
		t := i.(setLoger)
		t.SetLog(log)
		return nil
	})
}

func IsCapable() bool {
	if _, err := exec.LookPath("lvs"); err == nil {
		return true
	}
	return false
}

func hasMetad() bool {
	cmd := command.New(
		command.WithName("pgrep"),
		command.WithVarArgs("metad"),
	)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func pvscan(log *zerolog.Logger) error {
	args := make([]string, 0)
	if hasMetad() {
		args = append(args, "--cache")
	}
	cmd := command.New(
		command.WithName("pvscan"),
		command.WithArgs(args),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
	)
	return cmd.Run()
}
