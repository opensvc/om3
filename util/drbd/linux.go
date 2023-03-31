//go:build linux

package drbd

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"

	"github.com/opensvc/om3/util/command"
	"github.com/pkg/errors"
)

const (
	drbdadm = "/sbin/drbdadm"
	kmod    = "drbd"
)

func IsCapable() bool {
	if _, err := exec.LookPath(drbdadm); err != nil {
		return false
	}
	if !hasKMod(kmod) {
		return false
	}
	return true
}

func hasKMod(s string) bool {
	cmd := command.New(
		command.WithName("modinfo"),
		command.WithVarArgs(s),
	)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func Version() (string, error) {
	cmd := command.New(
		command.WithName(drbdadm),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	r := bytes.NewReader(b)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "Version: ") {
			return s[8:], nil
		}
	}
	return "", errors.Errorf("could not parse drbd version")
}
