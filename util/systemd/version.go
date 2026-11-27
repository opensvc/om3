package systemd

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ParseVersion reads the release of systemd in the output of "systemctl
// --version", whose first line is "systemd 249 (249.11-0ubuntu3.23)": the
// release is the number, what a distribution adds to it is not read.
func ParseVersion(s string) (int, error) {
	line, _, _ := strings.Cut(s, "\n")
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "systemd" {
		return 0, fmt.Errorf("no systemd version in %q", line)
	}
	// A release candidate is "systemd 256~rc3", and a point release
	// "systemd 255.4": the release is the leading digits.
	digits := fields[1]
	if i := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
		digits = digits[:i]
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("no systemd version in %q", line)
	}
	return n, nil
}

// Version returns the release of the systemd of the node, as 249, and an
// error when the node has no systemctl to ask.
func Version(ctx context.Context) (int, error) {
	b, err := exec.CommandContext(ctx, "systemctl", "--version").Output()
	if err != nil {
		return 0, fmt.Errorf("systemctl --version: %w", err)
	}
	return ParseVersion(string(b))
}
