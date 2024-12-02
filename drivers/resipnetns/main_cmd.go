package resipnetns

import (
	"fmt"

	"github.com/opensvc/om3/util/command"
	"github.com/rs/zerolog"
)

func (t *T) sysctlEnableIPV6In(dev, path string) error {
	cmd := command.New(
		command.WithName("nsenter"),
		command.WithArgs([]string{"--net=" + path, "sysctl", "-w", fmt.Sprintf("net.ipv6.conf.%s.disable_ipv6=0", dev)}),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
	)
	return cmd.Run()
}

func (t *T) linkDel(dev string) error {
	args := []string{"ip", "link", "del", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t *T) linkDelIn(dev, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "del", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t *T) linkSetNsPid(dev string, pid int) error {
	args := []string{"ip", "link", "set", dev, "netns", fmt.Sprint(pid)}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t *T) linkSetNameIn(dev, name, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "set", dev, "name", name}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t *T) linkSetUpIn(dev, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "set", dev, "up"}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
