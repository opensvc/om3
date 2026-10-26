package resfshost

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
)

// zoneContainer is a container resource whose root directory a filesystem
// can be mounted under, as the container.lxc one.
type zoneContainer interface {
	ContainerName() string
	ContainerHead(ctx context.Context) (string, error)
}

// ZoneMountPoint returns the mount point mnt of a filesystem of the object
// o, reparented into the root directory of the container zone names, when
// set, as v2 did: a container resource of the object holding a root
// directory, named by its name or its rid.
//
// A zone naming no such container is an error, rather than the filesystem
// mounted on the host path, outside the container it was meant for.
func ZoneMountPoint(ctx context.Context, o resource.ObjectDriver, zone, mnt string) (string, error) {
	mnt = filepath.Clean(mnt)
	if zone == "" {
		return mnt, nil
	}
	if o == nil {
		return "", fmt.Errorf("zone %s: no object to find the container in", zone)
	}
	for _, r := range o.ResourcesByDrivergroups([]driver.Group{driver.GroupContainer}) {
		c, ok := r.(zoneContainer)
		if !ok {
			continue
		}
		if c.ContainerName() != zone && r.RID() != zone {
			continue
		}
		root, err := c.ContainerHead(ctx)
		if err != nil {
			return "", fmt.Errorf("zone %s: %w", zone, err)
		}
		return filepath.Join(root, mnt), nil
	}
	return "", fmt.Errorf("zone %s: no container resource of the object holding a root directory is named so", zone)
}

// validateMountPoint refuses a relative mount point: it would resolve
// against the working directory of whichever process runs the action, the
// daemon or a command started anywhere, and the status, comparing it to the
// absolute paths of the mount table, would never see it mounted. With a
// zone, the mount point is the one in the container, absolute too.
func validateMountPoint(s string) error {
	if !filepath.IsAbs(s) {
		return fmt.Errorf("%s is not an absolute path", s)
	}
	return nil
}
