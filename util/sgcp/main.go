// Package sgcp provides utility functions for SGCP API interactions.
// It uses a YAML configuration file to avoid hardcoding API paths in the code.
package sgcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// IsDisabled returns true when the sgcp support is administratively disabled,
// which an operator declares by creating the disabled_flag file named by the
// configuration. A relative flag path is resolved against dir.
//
// A flag that can be neither confirmed present nor absent, because of a
// permission error on a parent directory or an io error, is reported as an
// error and not as a disable. The callers act on that error, where reading it
// as "disabled" would have them quietly skip the work they were asked to do.
func IsDisabled(dir string) (bool, error) {
	cfg := GetConfig()
	if cfg == nil || cfg.DisabledFlag == "" {
		return false, nil
	}
	path := cfg.DisabledFlag
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	switch _, err := os.Stat(path); {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("stat the sgcp disabled flag: %w", err)
	}
}
