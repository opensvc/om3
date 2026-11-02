package object

import (
	"context"
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/key"
)

const (
	// usageWalkInterval is how long a usage measured by walking a directory
	// is kept: a walk reads every entry of the tree, which a status
	// evaluation is not to do each time.
	usageWalkInterval = time.Hour

	// usageWalkBudget bounds a walk: a tree too large to walk in it has no
	// usage said, rather than a status evaluation held that long.
	usageWalkBudget = 5 * time.Second
)

var errUsageWalkBudget = errors.New("walk budget exceeded")

// volumeUsage returns what the volume holds on this node, nil when it is no
// volume, not up, or has no filesystem.
//
// A volume on a filesystem of its own, or on a directory a project quota
// bounds, is measured each time, which is cheap. A directory of a pool with
// no filesystem of its own shares the one of the node: it is walked, and the
// figure kept for usageWalkInterval, from the status before.
func (t *actor) volumeUsage(ctx context.Context, avail status.T) *instance.VolumeUsage {
	if t.path.Kind != naming.KindVol || avail != status.Up {
		return nil
	}
	r, head := t.volumeHeadResource()
	if head == "" {
		return nil
	}
	now := time.Now()
	if q, ok := r.(resource.ProjectQuotaUser); ok {
		if used, hard, ok, err := q.ProjectQuotaUsage(ctx); err == nil && ok {
			return &instance.VolumeUsage{Used: used, Size: hard, MeasuredAt: now}
		}
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(head, &st); err != nil {
		return nil
	}
	if isMountPoint(head) {
		bsize := int64(st.Bsize)
		return &instance.VolumeUsage{
			Used:       (int64(st.Blocks) - int64(st.Bfree)) * bsize,
			Size:       int64(st.Blocks) * bsize,
			MeasuredAt: now,
		}
	}
	if prev, err := t.statusLoad(); err == nil && prev.Usage != nil && now.Sub(prev.Usage.MeasuredAt) < usageWalkInterval {
		return prev.Usage
	}
	used, err := walkUsage(head, now.Add(usageWalkBudget))
	if err != nil {
		return nil
	}
	size := int64(st.Blocks) * int64(st.Bsize)
	if v := t.configuredVolumeSize(); v > 0 {
		size = v
	}
	return &instance.VolumeUsage{Used: used, Size: size, MeasuredAt: now}
}

// volumeHeadResource returns the resource holding the head of the volume, the
// shallowest mount point, and that mount point.
func (t *actor) volumeHeadResource() (resource.Driver, string) {
	type header interface{ Head() string }
	type candidate struct {
		r    resource.Driver
		head string
	}
	l := make([]candidate, 0)
	for _, r := range listResources(t) {
		switch r.ID().DriverGroup() {
		case driver.GroupFS, driver.GroupVolume:
		default:
			continue
		}
		if h, ok := r.(header); ok && h.Head() != "" {
			l = append(l, candidate{r, h.Head()})
		}
	}
	if len(l) == 0 {
		return nil, ""
	}
	sort.Slice(l, func(i, j int) bool { return l[i].head < l[j].head })
	return l[0].r, l[0].head
}

// configuredVolumeSize is the size the volume was claimed with, 0 when it
// says none.
func (t *actor) configuredVolumeSize() int64 {
	if size := t.config.GetSize(key.T{Section: "DEFAULT", Option: "size"}); size != nil {
		return *size
	}
	return 0
}

// isMountPoint says a filesystem is mounted on p: p is on another device than
// its parent, or the root.
func isMountPoint(p string) bool {
	var self, parent syscall.Stat_t
	if err := syscall.Lstat(p, &self); err != nil {
		return false
	}
	if err := syscall.Lstat(filepath.Dir(p), &parent); err != nil {
		return false
	}
	return self.Dev != parent.Dev || self.Ino == parent.Ino
}

// walkUsage returns what the regular files of the tree hold, by the blocks
// they take, as du counts them, before the deadline.
func walkUsage(head string, deadline time.Time) (int64, error) {
	root, err := os.OpenRoot(head)
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	var used int64
	err = iofs.WalkDir(root.FS(), ".", func(_ string, d iofs.DirEntry, err error) error {
		if time.Now().After(deadline) {
			return errUsageWalkBudget
		}
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			used += st.Blocks * 512
		} else if d.Type().IsRegular() {
			used += info.Size()
		}
		return nil
	})
	return used, err
}
