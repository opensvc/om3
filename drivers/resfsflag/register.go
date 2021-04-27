// +build linux solaris darwin

package resfsflag

import (
	"opensvc.com/opensvc/util/capabilities"
)

func capabilitiesScanner() ([]string, error) {
	return []string{"drivers.resource.fs.flag"}, nil
}

func init() {
	capabilities.Register(capabilitiesScanner)
}
