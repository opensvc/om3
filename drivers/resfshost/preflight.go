package resfshost

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/findmnt"
)

// ProvisionConflicts says what on the node holds what the provision would
// take: a filesystem mounted on the mount point, the device mounted, held by
// another device, or holding a signature the format would wipe.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	l := make([]string, 0)
	if mnt := t.mountPoint(); mnt != "" {
		if mounted, err := findmnt.HasMnt(ctx, mnt); err == nil && mounted {
			l = append(l, fmt.Sprintf("a filesystem is mounted on %s", mnt))
		}
	}
	dev := t.devpath(ctx)
	if !strings.HasPrefix(dev, "/dev/") || !file.Exists(dev) {
		return l
	}
	if mounted, err := findmnt.HasDev(ctx, dev); err == nil && mounted {
		l = append(l, fmt.Sprintf("%s is mounted", dev))
		return l
	}
	if holders, err := device.New(dev).Holders(); err == nil && len(holders) > 0 {
		l = append(l, fmt.Sprintf("%s is held by %s", dev, holders))
		return l
	}
	if sig := deviceSignature(ctx, dev); sig != "" {
		l = append(l, fmt.Sprintf("%s holds a %s signature", dev, sig))
	}
	return l
}

// deviceSignature returns the type of the signature dev holds, a filesystem,
// a volume group, a raid member, as blkid probes it, empty when none.
func deviceSignature(ctx context.Context, dev string) string {
	b, err := exec.CommandContext(ctx, "blkid", "-p", "-o", "value", "-s", "TYPE", dev).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
