package object

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// Migration is what a configuration has to become for om to run what it
	// describes, as the keyword changes a configuration update takes.
	//
	// It is expressed that way so that it lands through the path every other
	// configuration write takes: the same validation, the same policy, the
	// same claim on a pool.
	Migration struct {
		Sets   []keyop.T
		Unsets []key.T

		// Notes is what changed, in the words of whoever has to read the
		// configuration afterwards, one line per change.
		Notes []string

		// Refusals is what a rule recognized and could not write, with the
		// reason. A configuration keeps what nothing can migrate for it.
		Refusals []string
	}
)

// shareRegexp matches the share of a volume group a size can be written as in
// an om2 configuration: "60%FREE", "100%VG", "10%PVS".
var shareRegexp = regexp.MustCompile(`^([0-9]+)%(FREE|VG|PVS)$`)

// MigrateConfig is what a configuration has to become for om to run what it
// describes.
//
// A configuration written for om2 describes things om3 no longer reads that
// way. What it asked for is still possible, in another shape, and the shape
// is what this writes: the configuration says the same thing afterwards, in
// words om3 reads.
//
// It changes nothing by itself. What it answers is what a configuration
// update would take, so that the caller can show it before writing it.
func MigrateConfig(cfg *xconfig.T) Migration {
	var m Migration
	migrateFilesystemVolumes(cfg, &m)
	return m
}

// migrateFilesystemVolumes rewrites a filesystem that made the volume it
// mounts as a logical volume resource and a filesystem resting on it.
//
// The filesystem driver of om2 made the volume it mounted: the volume group
// to carve it from, the size to carve, and the options to carve it with were
// keywords of the filesystem. om3 describes the volume as a resource of its
// own, which is what makes it something the rest of om can see: a size it can
// grow, a device another resource can rest on, a claim a pool can count.
func migrateFilesystemVolumes(cfg *xconfig.T, m *Migration) {
	for _, section := range cfg.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() != driver.GroupFS {
			continue
		}
		moved := movedKeys(cfg, section)
		if len(moved) == 0 {
			continue
		}
		vg := cfg.Get(key.T{Section: section, Option: "vg"})
		dev := cfg.Get(key.T{Section: section, Option: "dev"})
		name := filepath.Base(dev)
		if vg == "" && strings.HasPrefix(dev, "/dev/") {
			// "/dev/<vg>/<lv>", which is where the volume group is named
			// when the keyword does not name it. Every other path under
			// /dev has two elements too, and says nothing about a volume
			// group: a device mapper name is not one.
			if l := strings.Split(strings.TrimPrefix(dev, "/dev/"), "/"); len(l) == 2 && l[0] != "mapper" {
				vg = l[0]
			}
		}
		if vg == "" || name == "" || name == dev {
			m.Refusals = append(m.Refusals, fmt.Sprintf("%s makes the volume it mounts, and neither %s.vg nor %s.dev says which volume group and volume: write them, or the disk.lv resource, by hand", section, section, section))
			continue
		}
		diskRID := freeDiskRID(cfg, rid.Index())
		m.Sets = append(m.Sets,
			set(diskRID, "type", "lv"),
			set(diskRID, "name", name),
			set(diskRID, "vg", vg),
		)
		for _, option := range moved {
			base, scope := cutScope(option)
			value := cfg.Get(key.T{Section: section, Option: option})
			switch base {
			case "vg":
				// Named on the disk already, and scoped where it was.
				if scope != "" {
					m.Sets = append(m.Sets, set(diskRID, option, value))
				}
			case "create_options":
				m.Sets = append(m.Sets, set(diskRID, option, value))
			case "size":
				converted, note := migrateSize(cfg, vg, value)
				m.Sets = append(m.Sets, set(diskRID, option, converted))
				if note != "" {
					m.Notes = append(m.Notes, note)
				}
			}
			m.Unsets = append(m.Unsets, key.T{Section: section, Option: option})
		}
		m.Sets = append(m.Sets, set(section, "dev", fmt.Sprintf("{%s.exposed_devs[0]}", diskRID)))
		m.Notes = append(m.Notes, fmt.Sprintf("%s made the %s volume of the %s group: %s makes it now, and %s mounts what it exposes", section, name, vg, diskRID, section))
	}
}

// migrateSize is the size a logical volume resource is given, from the size
// the filesystem was given.
//
// A share of a volume group is a size om cannot count: it is resolved by lvm,
// at create time, and whoever asks om how big the volume is gets no answer.
// Where the group is a resource of this object, the share becomes arithmetic
// on what that resource reports, which om resolves itself and can grow.
// Where it is not, the share is carried over as it stands: it still creates
// the volume it always created, and the configuration says so.
func migrateSize(cfg *xconfig.T, vg, size string) (string, string) {
	match := shareRegexp.FindStringSubmatch(size)
	if match == nil {
		return size, ""
	}
	rid := vgRID(cfg, vg)
	if rid == "" {
		return size, fmt.Sprintf("%s is a share of a volume group om does not hold, so it is kept as it is: om cannot say how big the volume is, and cannot grow it", size)
	}
	switch match[2] {
	case "FREE":
		return fmt.Sprintf("$(%s%% * {%s.free})", match[1], rid), ""
	case "VG":
		return fmt.Sprintf("$(%s%% * {%s.capacity})", match[1], rid), ""
	default:
		return size, fmt.Sprintf("%s is a share of the physical volumes of a group, which om does not report, so it is kept as it is", size)
	}
}

// vgRID is the resource of this object holding the volume group of a name,
// and is empty when no resource holds it.
func vgRID(cfg *xconfig.T, vg string) string {
	for _, section := range cfg.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() != driver.GroupDisk {
			continue
		}
		if cfg.Get(key.T{Section: section, Option: "type"}) != "vg" {
			continue
		}
		if cfg.Get(key.T{Section: section, Option: "name"}) == vg {
			return section
		}
	}
	return ""
}

// movedKeys is the keywords of a section that the filesystem driver no longer
// reads, in the order they are written.
func movedKeys(cfg *xconfig.T, section string) []string {
	l := make([]string, 0)
	for _, option := range cfg.Keys(section) {
		base, _ := cutScope(option)
		switch base {
		case "vg", "size", "create_options":
			l = append(l, option)
		}
	}
	return l
}

// freeDiskRID is a disk resource identifier this configuration does not use.
//
// The index of the filesystem is tried first, so that what belongs together
// reads together, and the name of the filesystem resource is what is fallen
// back to.
func freeDiskRID(cfg *xconfig.T, index string) string {
	for _, candidate := range []string{"disk#" + index, "disk#fs" + index} {
		if !cfg.HasSectionString(candidate) {
			return candidate
		}
	}
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("disk#fs%s%d", index, i)
		if !cfg.HasSectionString(candidate) {
			return candidate
		}
	}
}

func set(section, option, value string) keyop.T {
	return keyop.T{
		Key:   key.T{Section: section, Option: option},
		Op:    keyop.Set,
		Value: value,
	}
}

// cutScope splits a keyword written for a node into the keyword and the node.
func cutScope(option string) (string, string) {
	base, scope, _ := strings.Cut(option, "@")
	return base, scope
}
