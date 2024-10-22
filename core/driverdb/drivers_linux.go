//go:build linux

package driverdb

import (
	// Uncomment to load
	_ "github.com/opensvc/om3/drivers/networkbridge"
	_ "github.com/opensvc/om3/drivers/networklo"
	_ "github.com/opensvc/om3/drivers/networkroutedbridge"
	_ "github.com/opensvc/om3/drivers/pooldrbd"
	_ "github.com/opensvc/om3/drivers/poolloop"
	_ "github.com/opensvc/om3/drivers/poolvg"
	_ "github.com/opensvc/om3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/drivers/rescontainerdockercli"
	_ "github.com/opensvc/om3/drivers/rescontainerkvm"
	_ "github.com/opensvc/om3/drivers/rescontainerlxc"
	_ "github.com/opensvc/om3/drivers/rescontainerpodman"
	_ "github.com/opensvc/om3/drivers/rescontainervbox"
	_ "github.com/opensvc/om3/drivers/resdiskcrypt"
	_ "github.com/opensvc/om3/drivers/resdiskdrbd"
	_ "github.com/opensvc/om3/drivers/resdiskzpool"
	_ "github.com/opensvc/om3/drivers/resdiskzvol"
	_ "github.com/opensvc/om3/drivers/resipcni"
	_ "github.com/opensvc/om3/drivers/resipnetns"
)
