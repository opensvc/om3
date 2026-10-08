package rescontainerlxc

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/versioncmp"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	drvCap := drvID.Cap()
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("lxc-info"),
		command.WithVarArgs("--version"),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return l, nil
	}
	l = append(l, drvCap)
	if newer, err := versioncmp.Newer(string(b), "2.1"); err == nil && newer {
		l = append(l, drvCap+".cgroup_dir")
	}
	return l, nil
}
