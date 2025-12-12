package resiphost

import (
	"github.com/opensvc/om3/v3/util/command"
	"github.com/rs/zerolog"
)

func (t *T) addrAdd(addr, dev, label string) error {
	args := []string{"ip", "addr", "add", addr, "dev", dev}
	if label != "" {
		args = append(args, "label", label)
	}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) addrDel(addr, dev string) error {
	args := []string{"ip", "addr", "del", addr, "dev", dev}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}
