//go:build !nodrv
// +build !nodrv

package om

import (
	// Load all our generic and os specific drivers
	_ "github.com/opensvc/om3/core/driverdb"
)
