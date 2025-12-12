package rescontainerlxc

import (
	"strings"

	"github.com/hashicorp/go-version"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	drvCap := drvID.Cap()
	cmd := command.New(
		command.WithName("lxc-info"),
		command.WithVarArgs("--version"),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return l, nil
	}
	l = append(l, drvCap)
	vs := strings.TrimSpace(string(b))
	v, err := version.NewVersion(vs)
	if err != nil {
		return l, nil
	}
	constraints, err := version.NewConstraint("> 2.1")
	if err != nil {
		return l, nil
	}
	if constraints.Check(v) {
		l = append(l, drvCap+".cgroup_dir")
	}
	return l, nil
}
