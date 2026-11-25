package storeplakar

import (
	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/driver"
)

var (
	drvID = driver.NewID(driver.GroupStore, "plakar")

	_ backup.Driver   = (*T)(nil)
	_ backup.Streamer = (*T)(nil)
	_ backup.Pruner   = (*T)(nil)
	_ backup.Localer  = (*T)(nil)
	_ backup.Statser  = (*T)(nil)
	_ backup.Usager   = (*T)(nil)
)

func init() {
	driver.Register(drvID, New)
}

// New allocates a plakar store driver.
func New() backup.Driver {
	return &T{}
}
