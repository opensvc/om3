package hb

import (
	// Register hb drivers
	_ "github.com/opensvc/om3/v3/daemon/hb/hbdisk"
	_ "github.com/opensvc/om3/v3/daemon/hb/hbmcast"
	_ "github.com/opensvc/om3/v3/daemon/hb/hbrelay"
	_ "github.com/opensvc/om3/v3/daemon/hb/hbucast"
)
