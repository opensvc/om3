//go:build linux

package udevadm

import (
	"strings"

	"github.com/opensvc/om3/util/command"
)

func Settle() {
	cmd := command.New(
		command.WithName("udevadm"),
		command.WithVarArgs("settle"),
	)
	cmd.Run()
}

func Properties(dev string) (map[string]string, error) {
	m := make(map[string]string)
	cmd := command.New(
		command.WithName("udevadm"),
		command.WithVarArgs("info", "--query=property", "--name="+dev),
		command.WithOnStdoutLine(func(line string) {
			k, v, found := strings.Cut(line, "=")
			if found {
				m[k] = v
			}
		}),
	)
	err := cmd.Run()
	return m, err
}
