package chkfsidf

import "opensvc.com/opensvc/util/capabilities"

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	return []string{}, nil
}
