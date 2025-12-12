package scsi

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	SGPersistDriver struct {
		Log *plog.Logger
	}
)

// ReadRegistrations read the reservation from any operating path
func (t SGPersistDriver) ReadRegistrations(dev device.T) ([]string, error) {
	paths, err := dev.SCSIPaths()
	if err != nil {
		return []string{}, err
	}
	for _, path := range paths {
		if regs, err := t.readRegistrations(path); err == nil {
			return regs, nil
		}
	}
	return []string{}, fmt.Errorf("no operating path to read registrations on")
}

func (t SGPersistDriver) readRegistrations(dev device.T) ([]string, error) {
	l := make([]string, 0)
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("-n", "--in", "--read-keys", dev.Path()),
		command.WithBufferedStdout(),
		command.WithEnv(t.env("1")),
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

func (t SGPersistDriver) Register(dev device.T, key string) error {
	paths, err := dev.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.registerPath(path, key); err != nil {
			return err
		}
	}
	return nil
}

func (t SGPersistDriver) registerPath(dev device.T, key string) error {
	option := command.WithVarArgs("-n", "--out", "--register-ignore", "--param-sark", key, dev.Path())
	return t.retryOnUnitAttention(dev, option)
}

func (t SGPersistDriver) Unregister(dev device.T, key string) error {
	paths, err := dev.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.unregisterPath(path, key); err != nil {
			return err
		}
	}
	return nil
}

func (t SGPersistDriver) unregisterPath(dev device.T, key string) error {
	option := command.WithVarArgs("-n", "--out", "--register-ignore", "--param-rk", key, dev.Path())
	return t.retryOnUnitAttention(dev, option)
}

func (t SGPersistDriver) ReadReservation(dev device.T) (string, error) {
	paths, err := dev.SCSIPaths()
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		if key, err := t.readReservation(path); err == nil {
			return key, nil
		}
	}
	return "", fmt.Errorf("no operating path to read reservation on")
}

func (t SGPersistDriver) readReservation(dev device.T) (string, error) {
	cmd := command.New(
		command.WithName("sg_persist"),
		command.WithVarArgs("-n", "--in", "--read-reservation", dev.Path()),
		command.WithEnv(t.env("1")),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "    Key=0x") {
			return formatKey(line[8:]), nil
		}
	}
	return "", nil
}

func (t SGPersistDriver) Reserve(dev device.T, key string) error {
	var errs error
	paths, err := dev.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.reserve(path, key); err == nil {
			return nil
		} else {
			errs = errors.Join(errs, err)
		}
	}
	return fmt.Errorf("no %s path accepted reservation: %s", dev, errs)
}

func (t SGPersistDriver) reserve(dev device.T, key string) error {
	option := command.WithVarArgs("-n", "--out", "--reserve", "--param-rk", key, "--prout-type", DefaultPersistentReservationType, dev.Path())
	return t.retryOnUnitAttention(dev, option)
}

func (t SGPersistDriver) Release(dev device.T, key string) error {
	var errs error
	paths, err := dev.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.release(path, key); err == nil {
			return nil
		} else {
			errs = errors.Join(errs, err)
		}
	}
	return fmt.Errorf("no %s path accepted reservation release: %s", dev, errs)
}

func (t SGPersistDriver) release(dev device.T, key string) error {
	option := command.WithVarArgs("-n", "--out", "--release", "--param-rk", key, "--prout-type", DefaultPersistentReservationType, dev.Path())
	return t.retryOnUnitAttention(dev, option)
}

func (t SGPersistDriver) Clear(dev device.T, key string) error {
	var errs error
	paths, err := dev.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.clear(path, key); err == nil {
			return nil
		} else {
			errs = errors.Join(errs, err)
		}
	}
	return fmt.Errorf("no %s path accepted reservation clear: %s", dev, errs)
}

func (t SGPersistDriver) clear(dev device.T, key string) error {
	option := command.WithVarArgs("-n", "--out", "--clear", "--param-rk", key, dev.Path())
	return t.retryOnUnitAttention(dev, option)
}

func (t SGPersistDriver) Preempt(dev device.T, oldKey, newKey string) error {
	var errs error
	paths, err := dev.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.preempt(path, oldKey, newKey); err == nil {
			return nil
		} else {
			errs = errors.Join(errs, err)
		}
	}
	return fmt.Errorf("no %s path accepted reservation preempt: %s", dev, errs)
}

func (t SGPersistDriver) preempt(dev device.T, oldKey, newKey string) error {
	option := command.WithVarArgs("-n", "--out", "--preempt", "--param-sark", oldKey, "--param-rk", newKey, "--prout-type", DefaultPersistentReservationType, dev.Path())
	return t.retryOnUnitAttention(dev, option)
}

func (t SGPersistDriver) PreemptAbort(dev device.T, oldKey, newKey string) error {
	var errs error
	paths, err := dev.SCSIPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.preemptAbort(path, oldKey, newKey); err == nil {
			return nil
		} else {
			errs = errors.Join(errs, err)
		}
	}
	return fmt.Errorf("no %s path accepted reservation preempt-abort: %s", dev, errs)
}

func (t SGPersistDriver) preemptAbort(dev device.T, oldKey, newKey string) error {
	option := command.WithVarArgs("-n", "--out", "--preempt-abort", "--param-sark", oldKey, "--param-rk", newKey, "--prout-type", DefaultPersistentReservationType, dev.Path())
	return t.retryOnUnitAttention(dev, option)
}

// sgPersist returns the env vars to use with sg_persist commands
// to work with read-only devices.
func (t SGPersistDriver) env(val string) []string {
	return []string{
		"SG_PERSIST_O_RDONLY=" + val,
		"SG_PERSIST_IN_RDONLY=" + val, // sg_persist >= 1.39
	}
}

// ackUnitAttention does a --in command to acknowledge a unit attention, likely
// caused by the previous --out command.
func (t SGPersistDriver) ackUnitAttention(dev device.T) {
	t.Log.Tracef("ack Unit Attention on %s.", dev)
	_, _ = t.readReservation(dev)
}

func (t SGPersistDriver) retryOnUnitAttention(dev device.T, options ...funcopt.O) error {
	maxValue := 10
	countdown := maxValue
	for {
		options = append(
			options,
			command.WithName("sg_persist"),
			command.WithLogger(t.Log),
			command.WithCommandLogLevel(zerolog.InfoLevel),
			command.WithEnv(t.env("0")),
			command.WithBufferedStderr(),
			command.WithBufferedStdout(),
		)
		cmd := command.New(options...)
		t.ackUnitAttention(dev)
		err := cmd.Run()
		if err == nil {
			// all good
			t.Log.Attr("out", string(cmd.Stdout())).Tracef("")
			return err
		}
		if cmd.ExitCode() == 6 {
			if countdown == 1 {
				t.Log.Warnf("Unit Attention received from %s. max retries exhausted", dev)
				t.Log.Attr("out", string(cmd.Stdout())).Infof("")
				t.Log.Attr("err", string(cmd.Stderr())).Errorf("")
				return err
			}
			t.Log.Warnf("Unit Attention received from %s. ack and retry in 0.1s", dev)
			countdown--
			time.Sleep(100 * time.Millisecond)
			t.ackUnitAttention(dev)
			continue
		}
		// other exit codes are not retryable
		t.Log.Attr("out", string(cmd.Stdout())).Infof("")
		t.Log.Attr("err", string(cmd.Stderr())).Errorf("")
		return err

	}
}
