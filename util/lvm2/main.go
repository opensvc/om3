//go:build linux

package lvm2

import (
	"errors"
	"os/exec"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

type (
	driver struct {
		log *plog.Logger
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

func (t *driver) SetLog(log *plog.Logger) {
	t.log = log
}

func (t *driver) Log() *plog.Logger {
	return t.log
}

func WithLogger(log *plog.Logger) funcopt.O {
	type setLoger interface {
		SetLog(*plog.Logger)
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

func PVScan(log *plog.Logger) error {
	cmd := command.New(
		command.WithName("pvscan"),
		command.WithVarArgs("--cache"),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
	)
	return cmd.Run()
}
