package zfs

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
)

func datasetGetProperty(ds Dataset, prop string) (string, error) {
	cmd := command.New(
		command.WithName("/usr/sbin/zfs"),
		command.WithVarArgs("get", "-Hp", "-o", "value", prop, ds.GetName()),
		command.WithBufferedStdout(),
		command.WithLogger(ds.GetLog()),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func datasetSetProperty(ds Dataset, prop, value string) error {
	s := fmt.Sprintf("%s=%s", prop, value)
	cmd := command.New(
		command.WithName("/usr/sbin/zfs"),
		command.WithVarArgs("set", s, ds.GetName()),
		command.WithLogger(ds.GetLog()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
