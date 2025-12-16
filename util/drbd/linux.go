//go:build linux

package drbd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/opensvc/om3/v3/util/command"
)

const (
	drbdadm   = "/sbin/drbdadm"
	drbdsetup = "/sbin/drbdsetup"
	kmod      = "drbd"
)

func IsCapable(ctx context.Context) bool {
	if _, err := exec.LookPath(drbdadm); err != nil {
		return false
	}
	if !hasKMod(ctx, kmod) {
		return false
	}
	return true
}

func hasKMod(ctx context.Context, s string) bool {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("modinfo"),
		command.WithVarArgs(s),
	)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func Version(ctx context.Context) (string, error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(drbdadm),
		command.WithBufferedStdout(),
		command.WithIgnoredExitCodes(1),
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
			return s[9:], nil
		}
	}
	return "", fmt.Errorf("could not parse drbd version")
}
