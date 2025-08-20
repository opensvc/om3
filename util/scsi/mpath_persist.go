//go:build linux

package scsi

import (
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/plog"
)

type (
	MpathPersistDriver struct {
		Log *plog.Logger
	}
)

func (t MpathPersistDriver) ReadRegistrations(dev device.T) ([]string, error) {
	l := make([]string, 0)
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--in", "--read-keys", dev.Path()),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return l, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "    0x") {
			l = append(l, formatKey(line[4:]))
		}
	}
	return l, nil
}

func (t MpathPersistDriver) Register(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--out", "--register-ignore", "--param-sark", key, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t MpathPersistDriver) Unregister(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--out", "--register-ignore", "--param-rk", key, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t MpathPersistDriver) ReadReservation(dev device.T) (string, error) {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--in", "--read-reservation", dev.Path()),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "   Key = 0x") {
			return formatKey(line[9:]), nil
		}
	}
	return "", nil
}

func (t MpathPersistDriver) Reserve(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--out", "--reserve", "--param-rk", key, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t MpathPersistDriver) Release(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--out", "--release", "--param-rk", key, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t MpathPersistDriver) Clear(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--out", "--clear", "--param-rk", key, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t MpathPersistDriver) Preempt(dev device.T, oldKey, newKey string) error {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--out", "--preempt", "--param-sark", oldKey, "--param-rk", newKey, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t MpathPersistDriver) PreemptAbort(dev device.T, oldKey, newKey string) error {
	cmd := command.New(
		command.WithName("mpathpersist"),
		command.WithVarArgs("--out", "--preempt-abort", "--param-sark", oldKey, "--param-rk", newKey, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
