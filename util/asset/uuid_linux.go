//go:build linux

package asset

import (
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// machineUUID returns the SMBIOS system uuid of the node, lowercased, as
// /sys/class/dmi/id/product_uuid says it. A hypervisor sets it to the uuid
// of the virtual machine, as libvirt to the <uuid> of the domain, so it
// links a guest node to the container resource running it.
//
// A node with no dmi, or whose firmware sets no uuid, has none: the
// property is then left unset rather than reported with a placeholder.
// Reading the file needs root.
func machineUUID() (string, error) {
	s := strings.ToLower(readTrimmed(filepath.Join(serialSources.dmiDir, "product_uuid")))
	u, err := uuid.Parse(s)
	if err != nil {
		return "", ErrIgnore
	}
	switch u {
	case uuid.Nil, uuid.Max:
		return "", ErrIgnore
	}
	return u.String(), nil
}
