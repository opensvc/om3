package storeplakar

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

// capabilitiesScanner publishes the driver capability and the path of the
// plakar binary when one is in the PATH. Without it, the store is not
// usable on this node.
func capabilitiesScanner(ctx context.Context) ([]string, error) {
	p, err := exec.LookPath("plakar")
	if err != nil {
		return []string{}, nil
	}
	return []string{
		drvID.Cap(),
		capabilities.MakePath("plakar", p),
	}, nil
}
