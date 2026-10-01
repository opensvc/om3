package rescontainerkvm

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/xmlquery"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/device"
)

type (
	// moveDisk is a disk of the domain: the name the guest knows it by,
	// and its source on the host, a device or a file.
	moveDisk struct {
		Target string
		Source string
		IsFile bool

		// Format is the format of the image the source holds, raw when
		// the definition says none.
		Format string

		// Copyable says the disk can be mirrored by a migration: a
		// read-only disk, a cdrom image among them, or a disk shared
		// with other domains is not.
		Copyable bool
	}
)

// parseMoveDisks returns the disks of a domain definition that have a
// source on the host.
func parseMoveDisks(r io.Reader) ([]moveDisk, error) {
	doc, err := xmlquery.Parse(r)
	if err != nil {
		return nil, err
	}
	es, err := xmlquery.QueryAll(doc, "//domain/devices/disk")
	if err != nil {
		return nil, err
	}
	l := make([]moveDisk, 0, len(es))
	for _, e := range es {
		d := moveDisk{Copyable: true}
		if target := xmlquery.FindOne(e, "target"); target != nil {
			d.Target = target.SelectAttr("dev")
		}
		if driver := xmlquery.FindOne(e, "driver"); driver != nil {
			d.Format = driver.SelectAttr("type")
		}
		if source := xmlquery.FindOne(e, "source"); source != nil {
			if v := source.SelectAttr("dev"); v != "" {
				d.Source = v
			} else if v := source.SelectAttr("file"); v != "" {
				d.Source = v
				d.IsFile = true
			}
		}
		if d.Source == "" {
			// A network disk, or an empty cdrom drive, is on no
			// resource of the object.
			continue
		}
		if xmlquery.FindOne(e, "readonly") != nil || xmlquery.FindOne(e, "shareable") != nil {
			d.Copyable = false
		}
		l = append(l, d)
	}
	return l, nil
}

// moveDisks returns the disks of the domain that have a source on the host.
func (t *T) moveDisks() ([]moveDisk, error) {
	f, err := os.Open(t.configFile())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMoveDisks(f)
}

// moveResources returns the resources holding the disks of the domain, each
// once, in the order of the disks, and the disks a migration copies: the ones
// on a resource whose storage the destination can not reach.
//
// A disk on a device is held by the resource exposing that device, as a
// drbd or a zvol. A disk in a file is held by the filesystem it is in.
func (t *T) moveResources(ctx context.Context) ([]resource.Driver, []moveDisk, error) {
	disks, err := t.moveDisks()
	if err != nil {
		return nil, nil, fmt.Errorf("read the disks of %s: %w", t.Name, err)
	}
	obj, err := t.obj()
	if err != nil {
		return nil, nil, err
	}
	actor := obj.(object.Actor)
	var (
		resources []resource.Driver
		copyDisks []moveDisk
		seen      = make(map[string]bool)
	)
	for _, d := range disks {
		var r resource.Driver
		if d.IsFile {
			r, err = actor.ResourceHandlingFile(ctx, d.Source)
		} else {
			r, err = actor.ResourceHandlingDevice(ctx, device.New(d.Source))
		}
		if err != nil {
			return nil, nil, err
		}
		if r == nil || r.IsDisabled() {
			continue
		}
		if i, ok := r.(resource.MoveStorageCopier); ok && i.MoveCopiesStorage() {
			if !d.Copyable {
				t.Log().Infof("disk %s is not mirrored by the migration: %s is read-only or shared, and is copied as a file of %s", d.Target, d.Source, r.RID())
			} else {
				copyDisks = append(copyDisks, d)
			}
		}
		if seen[r.RID()] {
			continue
		}
		seen[r.RID()] = true
		resources = append(resources, r)
	}
	return resources, copyDisks, nil
}

// migrateArgs returns the virsh arguments migrating the domain name to
// toURI. The disks named in copyDisks are mirrored to the destination while
// the domain runs, and only those: the others are shared with it.
//
// With syncWrites, a mirrored disk is written on both nodes before the guest
// is told its write is done, once the mirror has caught up. A guest writing
// faster than the link carries otherwise keeps the mirror from ever catching
// up, and the migration from ending. Proxmox switches its mirrors the same
// way.
//
// With overlaid, the copied disks are overlays over a copy of the disk the
// destination already holds, and only the overlays are copied.
func migrateArgs(name, toURI string, copyDisks []string, overlaid, syncWrites bool) []string {
	args := []string{"migrate", "--live", "--persistent"}
	if len(copyDisks) > 0 {
		if overlaid {
			args = append(args, "--copy-storage-inc")
		} else {
			args = append(args, "--copy-storage-all")
		}
		args = append(args, "--migrate-disks", strings.Join(copyDisks, ","))
		if syncWrites {
			args = append(args, "--copy-storage-synchronous-writes")
		}
	}
	return append(args, name, toURI)
}

var (
	virshSyncWritesOnce sync.Once
	virshSyncWrites     bool
)

// virshHasSyncWrites says whether virsh migrate takes
// --copy-storage-synchronous-writes, which libvirt 8.0 added. An older virsh
// refuses an option it does not know, and the migration with it.
func virshHasSyncWrites(ctx context.Context) bool {
	virshSyncWritesOnce.Do(func() {
		b, err := exec.CommandContext(ctx, "virsh", "help", "migrate").Output()
		virshSyncWrites = err == nil && hasSyncWritesOption(string(b))
	})
	return virshSyncWrites
}

func hasSyncWritesOption(help string) bool {
	return strings.Contains(help, "--copy-storage-synchronous-writes")
}

// migrateTimeout returns the longest a migration may take, zero for no limit
// of its own. See the migrate_timeout keyword.
//
// A migration copying disks takes the time to copy them, which the
// stop_timeout of a guest, meant for its shutdown and two minutes by default,
// has nothing to say about: a guest with large disks would see its move
// cancelled and rolled back. The stop action of the object still bounds it.
func migrateTimeout(migrate, stop *time.Duration, copiesDisks bool) time.Duration {
	switch {
	case migrate != nil:
		return *migrate
	case copiesDisks:
		return 0
	case stop != nil:
		return *stop
	default:
		return 0
	}
}
