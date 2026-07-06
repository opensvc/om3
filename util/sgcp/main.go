// Package sgcp provides utility functions for SGCP API interactions.
// It uses a YAML configuration file to avoid hardcoding API paths in the code.
package sgcp

import (
	"github.com/opensvc/om3/v3/util/file"
)

func IsDisabled(s string) bool {
	if config.DisabledFlag == "" {
		return false
	}
	return file.Exists(config.DisabledFlag)
}
