package resiphost

import (
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
)

func (t *T) addrAdd(addr, dev, label string) error {
	args := []string{"ip", "addr", "add", addr, "dev", dev}
	if label != "" {
		args = append(args, "label", label)
	}
	if ip := t.ipaddr(); ip != nil && ip.To4() == nil {
		// Usable at once, rather than tentative for the second duplicate
		// address detection takes, in which a service binding the address
		// fails: the start already checked nothing answers it.
		args = append(args, "nodad")
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
