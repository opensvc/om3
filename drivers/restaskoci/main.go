package rescontaineroci

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/restaskdocker"
	"github.com/opensvc/om3/v3/drivers/restaskpodman"
	"github.com/opensvc/om3/v3/util/capabilities"
)

var (
	drvID = driver.NewID(driver.GroupTask, "oci")
)

func New() resource.Driver {
	if capabilities.Has(restaskdocker.DrvID.Cap()) {
		return restaskdocker.New()
	}
	return restaskpodman.New()
}

func init() {
	driver.Register(drvID, New)
}
