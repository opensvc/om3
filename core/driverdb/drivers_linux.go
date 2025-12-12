//go:build linux

package driverdb

import (
	// Uncomment to load
	_ "github.com/opensvc/om3/v3/drivers/networkbridge"
	_ "github.com/opensvc/om3/v3/drivers/networklo"
	_ "github.com/opensvc/om3/v3/drivers/networkroutedbridge"
	_ "github.com/opensvc/om3/v3/drivers/pooldrbd"
	_ "github.com/opensvc/om3/v3/drivers/poolloop"
	_ "github.com/opensvc/om3/v3/drivers/poolvg"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerkvm"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerlxc"
	_ "github.com/opensvc/om3/v3/drivers/rescontaineroci"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerpodman"
	_ "github.com/opensvc/om3/v3/drivers/rescontainervbox"
	_ "github.com/opensvc/om3/v3/drivers/resdiskcrypt"
	_ "github.com/opensvc/om3/v3/drivers/resdiskdrbd"
	_ "github.com/opensvc/om3/v3/drivers/resdiskzpool"
	_ "github.com/opensvc/om3/v3/drivers/resdiskzvol"
	_ "github.com/opensvc/om3/v3/drivers/resipcni"
	_ "github.com/opensvc/om3/v3/drivers/resipnetns"
	_ "github.com/opensvc/om3/v3/drivers/restaskdocker"
	_ "github.com/opensvc/om3/v3/drivers/restaskoci"
	_ "github.com/opensvc/om3/v3/drivers/restaskpodman"
)
