package rescontaineroci

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/util/capabilities"
)

var (
	drvID = driver.NewID(driver.GroupContainer, "oci")
)

func New() resource.Driver {
	if capabilities.Has(rescontainerdocker.DrvID.Cap()) {
		return rescontainerdocker.New()
	}
	return rescontainerpodman.New()
}

func init() {
	driver.Register(drvID, New)
}
