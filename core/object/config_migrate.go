package object

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
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
	migrateTmpfsMountOptions(cfg, &m)
	return m
}

// tmpfsSizeRegexp matches the size option of a tmpfs mount, as the kernel
// parses it: a number of bytes, or of kibi, mebi, gibi, tebi, pebi or exbi
// bytes. A share of the memory, "50%", is a size too, and one om cannot say.
var tmpfsSizeRegexp = regexp.MustCompile(`^([0-9]+)([kKmMgGtTpPeE]?)$`)

// tmpfsModeRegexp matches the mode option of a tmpfs mount, in octal.
var tmpfsModeRegexp = regexp.MustCompile(`^[0-7]{3,4}$`)

// migrateTmpfsMountOptions moves the size and the mode of a tmpfs from its
// mount options to its size and mode keywords.
//
// The size of a tmpfs is what a resize grows, and a resize records the size
// it reached in the size keyword of the resource. Written in mnt_opt, the size
// stayed the one the tmpfs was made with, and the next mount undid the
// resize.
//
// The tmpfs of a volume, which a shm pool made, is sized by reference to
// DEFAULT.size, the size the volume is claimed with, as the pool makes it
// now: a resize of the volume records there. Where the two said different
// sizes, the volume is mounted at DEFAULT.size from then on, which is what a
// resize asked for and a remount did not keep.
func migrateTmpfsMountOptions(cfg *xconfig.T, m *Migration) {
	isVolume := cfg.HasKey(key.T{Section: "DEFAULT", Option: "size"})
	for _, section := range cfg.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() != driver.GroupFS {
			continue
		}
		if cfg.Get(key.T{Section: section, Option: "type"}) != "tmpfs" {
			continue
		}
		for _, option := range cfg.Keys(section) {
			base, scope := cutScope(option)
			if base != "mnt_opt" {
				continue
			}
			migrateTmpfsMountOption(cfg, m, section, scope, isVolume)
		}
	}
}

func migrateTmpfsMountOption(cfg *xconfig.T, m *Migration, section, scope string, isVolume bool) {
	scoped := func(option string) string {
		if scope == "" {
			return option
		}
		return option + "@" + scope
	}
	mntOptKey := key.T{Section: section, Option: scoped("mnt_opt")}
	kept := make([]string, 0)
	var changed bool
	for _, opt := range strings.Split(cfg.Get(mntOptKey), ",") {
		name, value, _ := strings.Cut(opt, "=")
		switch name {
		case "size":
			sizeKey := key.T{Section: section, Option: scoped("size")}
			if cfg.HasKey(sizeKey) {
				m.Notes = append(m.Notes, fmt.Sprintf("%s: %s is dropped from %s: the size keyword sets the size, and the mount ignored it", section, opt, mntOptKey))
				changed = true
				continue
			}
			bytes, ok := tmpfsSize(value)
			if !ok {
				m.Refusals = append(m.Refusals, fmt.Sprintf("%s: %s is kept in %s: om cannot say how big a share of the memory is, so it cannot record a resize in it", section, opt, mntOptKey))
				kept = append(kept, opt)
				continue
			}
			sized := sizeconv.ExactBSizeCompact(float64(bytes))
			if isVolume {
				m.Sets = append(m.Sets, set(section, scoped("size"), "{DEFAULT.size}"))
				if claimed, err := sizeconv.FromSize(cfg.Get(key.T{Section: "DEFAULT", Option: "size"})); err == nil && claimed != bytes {
					m.Notes = append(m.Notes, fmt.Sprintf("%s was mounted at %s, and the volume is configured at %s: it is mounted at %s from now on", section, sized, sizeconv.ExactBSizeCompact(float64(claimed)), sizeconv.ExactBSizeCompact(float64(claimed))))
				}
			} else {
				m.Sets = append(m.Sets, set(section, scoped("size"), sized))
			}
			m.Notes = append(m.Notes, fmt.Sprintf("%s: %s of %s is the size keyword now, which a resize records in", section, opt, mntOptKey))
			changed = true
		case "mode":
			modeKey := key.T{Section: section, Option: scoped("mode")}
			if cfg.HasKey(modeKey) {
				m.Notes = append(m.Notes, fmt.Sprintf("%s: %s is dropped from %s: the mode keyword sets the mode, and the mount ignored it", section, opt, mntOptKey))
				changed = true
				continue
			}
			if !tmpfsModeRegexp.MatchString(value) {
				m.Refusals = append(m.Refusals, fmt.Sprintf("%s: %s is kept in %s: it is not a mode in octal", section, opt, mntOptKey))
				kept = append(kept, opt)
				continue
			}
			m.Sets = append(m.Sets, set(section, scoped("mode"), value))
			m.Notes = append(m.Notes, fmt.Sprintf("%s: %s of %s is the mode keyword now", section, opt, mntOptKey))
			changed = true
		case "":
		default:
			kept = append(kept, opt)
		}
	}
	if !changed {
		return
	}
	if len(kept) == 0 {
		m.Unsets = append(m.Unsets, mntOptKey)
	} else {
		m.Sets = append(m.Sets, set(section, mntOptKey.Option, strings.Join(kept, ",")))
	}
}

// tmpfsSize is the size in bytes of the size option of a tmpfs mount, false
// for a share of the memory.
func tmpfsSize(s string) (int64, bool) {
	match := tmpfsSizeRegexp.FindStringSubmatch(s)
	if match == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, false
	}
	shift := map[string]uint{"": 0, "k": 10, "m": 20, "g": 30, "t": 40, "p": 50, "e": 60}[strings.ToLower(match[2])]
	return n << shift, true
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
		if !madeItsVolume(moved) {
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

// madeItsVolume reports whether the keywords of a filesystem say om2 made the
// volume it mounts: a volume group to carve it from, and a size to carve.
//
// It is the condition om2 made a volume on, a volume group being named, and
// the size is what it carved. A size alone is not the om2 one: it is the size
// a tmpfs or a quota-capped directory may hold, which their om3 drivers read
// as their own.
func madeItsVolume(moved []string) bool {
	var hasSize, hasVG bool
	for _, option := range moved {
		switch base, _ := cutScope(option); base {
		case "size":
			hasSize = true
		case "vg":
			hasVG = true
		}
	}
	return hasSize && hasVG
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
