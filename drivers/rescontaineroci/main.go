package rescontaineroci

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/v3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/v3/util/capabilities"
)

var (
	drvID = driver.NewID(driver.GroupContainer, "oci")
)

func New() resource.Driver {
	// The objects map the oci type to the engine of the node, see
	// Node.OCIEngine: this one serves what has no node at hand, as the
	// keywords of the type, in the same order, podman first, as in v2.
	if capabilities.Has(rescontainerpodman.DrvID.Cap()) {
		return rescontainerpodman.New()
	}
	return rescontainerdocker.New()
}

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Runs an OCI container with the engine the node.oci keyword names, else the first the node has, podman then docker.")
	driver.SetSelects(drvID, rescontainerpodman.DrvID, rescontainerdocker.DrvID)
}
