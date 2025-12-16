package ressyncsymsnapvx

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	baseCap := drvID.Cap()
	l := make([]string, 0)
	if _, err := exec.LookPath("symsnapvx"); err == nil {
		l = append(l, baseCap)
	}
	return l, nil
}
