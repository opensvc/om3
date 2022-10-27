package hb

import (
	// Register hb drivers
	_ "opensvc.com/opensvc/daemon/hb/hbdisk"
	_ "opensvc.com/opensvc/daemon/hb/hbmcast"
	_ "opensvc.com/opensvc/daemon/hb/hbrelay"
	_ "opensvc.com/opensvc/daemon/hb/hbucast"
)
