package object

import (
	"slices"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/key"
)

// ociEngines are the engines an oci container or task runs with, in the
// order they are preferred when the node.oci keyword is not set, as in v2.
var ociEngines = []string{"podman", "docker"}

// OCIEngine returns the engine the oci container and task resources of the
// node run with: the node.oci keyword, else podman when the node has it,
// else docker, as in v2.
func (t *Node) OCIEngine() string {
	// A value no engine has is ignored, the validation of the
	// configuration saying so.
	if s := t.MergedConfig().GetString(key.New("node", "oci")); slices.Contains(ociEngines, s) {
		return s
	}
	for _, name := range ociEngines {
		if capabilities.Has(driver.NewID(driver.GroupContainer, name).Cap()) {
			return name
		}
	}
	return "docker"
}
