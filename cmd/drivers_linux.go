//go:build linux
// +build linux

package cmd

import (
	_ "opensvc.com/opensvc/drivers/networkbridge"
	_ "opensvc.com/opensvc/drivers/networklo"
	_ "opensvc.com/opensvc/drivers/networkroutedbridge"
	_ "opensvc.com/opensvc/drivers/poolvg"
	_ "opensvc.com/opensvc/drivers/rescontainerdocker"
	_ "opensvc.com/opensvc/drivers/rescontainerkvm"
	_ "opensvc.com/opensvc/drivers/rescontainerlxc"
	_ "opensvc.com/opensvc/drivers/resdiskcrypt"
	_ "opensvc.com/opensvc/drivers/resdiskzpool"
	_ "opensvc.com/opensvc/drivers/resdiskzvol"
	_ "opensvc.com/opensvc/drivers/resipcni"
	_ "opensvc.com/opensvc/drivers/resipnetns"
)
