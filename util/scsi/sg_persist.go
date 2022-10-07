package scsi

import (
	"strings"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/device"
)

type (
	SGPersistDriver struct {
		Log *zerolog.Logger
	}
)

func (t SGPersistDriver) ReadRegistrations(dev device.T) ([]string, error) {
	l := make([]string, 0)
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--in", "--read-keys", dev.Path()),
		command.WithBufferedStdout(),
		command.WithEnv(t.env("1")),
	)
	b, err := cmd.Output()
	if err != nil {
		return l, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "    0x") {
			l = append(l, line[6:])
		}
	}
	return l, nil
}

func (t SGPersistDriver) Register(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--out", "--register-ignore", "--param-sark", key, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithEnv(t.env("0")),
	)
	return cmd.Run()
}

func (t SGPersistDriver) Unregister(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--out", "--register-ignore", "--param-rk", key, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithEnv(t.env("0")),
	)
	return cmd.Run()
}

func (t SGPersistDriver) ReadReservation(dev device.T) (string, error) {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--in", "--read-reservation", dev.Path()),
		command.WithEnv(t.env("1")),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "   Key = 0x") {
			return line[11:], nil
		}
	}
	return "", nil
}

func (t SGPersistDriver) Reserve(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--out", "--reserve", "--param-rk", key, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithEnv(t.env("0")),
	)
	return cmd.Run()
}

func (t SGPersistDriver) Release(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--out", "--release", "--param-rk", key, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithEnv(t.env("0")),
	)
	return cmd.Run()
}

func (t SGPersistDriver) Clear(dev device.T, key string) error {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--out", "--clear", "--param-rk", key, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithEnv(t.env("0")),
	)
	return cmd.Run()
}

func (t SGPersistDriver) Preempt(dev device.T, oldKey, newKey string) error {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--out", "--preempt", "--param-sark", oldKey, "--param-rk", newKey, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithEnv(t.env("0")),
	)
	return cmd.Run()
}

func (t SGPersistDriver) PreemptAbort(dev device.T, oldKey, newKey string) error {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("--out", "--preempt-abort", "--param-sark", oldKey, "--param-rk", newKey, "--prout-type", DefaultPersistentReservationType, dev.Path()),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithEnv(t.env("0")),
	)
	return cmd.Run()
}

// sgPersist returns the env vars to use with sg_persist commands
// to work with read-only devices.
func (t SGPersistDriver) env(val string) []string {
	return []string{
		"SG_PERSIST_O_RDONLY=" + val,
		"SG_PERSIST_IN_RDONLY=" + val, // sg_persist >= 1.39
	}
}
