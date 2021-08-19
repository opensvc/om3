// +build linux

package filesystems

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/command"
)

func (t T) Mount(dev string, mnt string, options string) error {
	timeout, _ := time.ParseDuration("1m")
	args := []string{"-t", t.Type()}
	if len(options) > 0 {
		args = append(args, "-o", options)
	}
	args = append(args, dev, mnt)
	cmd := command.New(
		command.WithName("mount"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithTimeout(timeout),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}

func (t T) Umount(mnt string) error {
	timeout, _ := time.ParseDuration("1m")
	cmd := command.New(
		command.WithName("umount"),
		command.WithVarArgs(mnt),
		command.WithLogger(t.Log()),
		command.WithTimeout(timeout),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}
