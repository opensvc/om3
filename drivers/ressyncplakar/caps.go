package ressyncplakar

import (
	"context"
	"os"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

var (
	capsConfigdir = drvID.Cap() + ".configdir"
)

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	drvCap := drvID.Cap()
	l = append(l, drvCap)
	plakarPath, err := exec.LookPath(plakar)
	if err != nil {
		return []string{}, nil
	}
	l = append(l, capabilities.MakePath(plakar, plakarPath))
	tmp, err := os.MkdirTemp("", "plakar-capabilities")
	if err != nil {
		return []string{}, nil
	}
	defer os.RemoveAll(tmp)

	tryFlag := func(flag string) bool {
		cmd := exec.CommandContext(ctx, plakar, flag, tmp, "version")
		if err := cmd.Run(); err != nil {
			return false
		}
		return true
	}

	if tryFlag("-configdir") {
		l = append(l, capsConfigdir)
	}
	return l, nil
}
