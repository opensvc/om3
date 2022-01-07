// +build linux

package cmd

import (
	_ "opensvc.com/opensvc/drivers/poolvg"
	_ "opensvc.com/opensvc/drivers/rescontainerdocker"
	_ "opensvc.com/opensvc/drivers/resipcni"
	_ "opensvc.com/opensvc/drivers/resipnetns"
)
