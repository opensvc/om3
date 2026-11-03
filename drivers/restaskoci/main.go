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
	// The objects map the oci type to the engine of the node, see
	// Node.OCIEngine: this one serves what has no node at hand, as the
	// keywords of the type, in the same order, podman first, as in v2.
	if capabilities.Has(restaskpodman.DrvID.Cap()) {
		return restaskpodman.New()
	}
	return restaskdocker.New()
}

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Runs a command in an OCI container, with the engine the node.oci keyword names, else the first the node has, podman then docker, as a task.")
	driver.SetSelects(drvID, restaskpodman.DrvID, restaskdocker.DrvID)
}
