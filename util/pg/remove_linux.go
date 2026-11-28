//go:build linux

package pg

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
)

// systemControlDirs are where the systemd of the system keeps the properties
// set with systemctl set-property, at runtime and for good. They are
// replaced by the tests.
var systemControlDirs = []string{"/run/systemd/system.control", "/etc/systemd/system.control"}

// legacyRuntimeDir is where a systemd older than 236 keeps the properties
// set at runtime, among the runtime units. It is replaced by the tests.
var legacyRuntimeDir = "/run/systemd/system"

// userControlDir is where the systemd instance of a user keeps the
// properties set at runtime. It is replaced by the tests.
var userControlDir = func(uid int) string {
	return fmt.Sprintf("/run/user/%d/systemd/user.control", uid)
}

// RemoveTree removes the group and every group under it, and has systemd
// forget the properties set on their slices.
//
// It is what ends an unscoped stop, shutdown or unprovision, as the v2 agent
// did: the cappings are written in the groups and in the properties of their
// slices, and both outlive the processes. An object created again under the
// same name, with no pg keyword, ran capped by what the previous one left.
//
// The groups are removed bottom-up, from every hierarchy the node holds: the
// one tree of the unified hierarchy, each controller of the v1 one, and the
// trees systemd delegates to the users, where the groups of a rootless
// container are. They are found on the filesystem, not from the groups an
// action registered: a stop applies none, and the groups of the subsets and
// resources were never registered by it. A group still holding a process
// cannot be removed, and the failure is not one: it stays, with the groups
// above it.
//
// The properties are reverted for the slices whose group is gone, and kept
// for the ones still there, with their group: removing one without the other
// leaves a capping, the group keeping its files, or systemd writing the
// properties again in the group it makes for the slice the next time. A
// systemd too old to revert is made to forget them, see forgetV1Properties.
//
// On the v1 hierarchy, the slices that had properties and whose group is
// gone are stopped too, see stopSlices.
//
// Only the group and the ones under it are touched. The groups above it,
// those of the namespace and the one holding every object of the node, are
// shared with other objects.
func (c Config) RemoveTree() error {
	if strings.Trim(c.ID, "/") == "" {
		return fmt.Errorf("pg remove: refusing to remove the root group")
	}
	kept := make(map[string]bool)
	for _, root := range hierarchyRoots() {
		c.removeGroups(filepath.Join(root, c.ID), kept)
	}
	// The slices to stop are read before the revert, which removes what
	// they are found from.
	units := make([]string, 0)
	if !isUnified() {
		units = c.slicesWithDropIns(systemControlDirs, kept)
	}
	errs := c.revertProperties(nil, systemControlDirs, kept)
	if !isUnified() {
		forgotten, err := c.forgetV1Properties(kept)
		errs = errors.Join(errs, err, c.stopSlices(append(units, forgotten...)))
	}
	if isUnified() {
		for _, d := range userDelegations() {
			userKept := make(map[string]bool)
			c.removeGroups(filepath.Join(UnifiedPath(), d.Root, c.ID), userKept)
			errs = errors.Join(errs, c.revertProperties(&d, []string{userControlDir(d.UID)}, userKept))
		}
	}
	return errs
}

// hierarchyRoots are the cgroup hierarchies of the node: the unified one, or
// every one mounted under the cgroup mount, v1 controllers, the systemd named
// hierarchy and the unified one of a hybrid node. The links to them, as cpu
// to cpu,cpuacct, are not followed, the hierarchy being there already.
func hierarchyRoots() []string {
	if isUnified() {
		return []string{UnifiedPath()}
	}
	entries, err := os.ReadDir(cgroupMount)
	if err != nil {
		return nil
	}
	l := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			l = append(l, filepath.Join(cgroupMount, e.Name()))
		}
	}
	return l
}

// removeGroups removes top and the groups under it, bottom-up, and adds to
// kept the name of each one it could not remove.
func (c Config) removeGroups(top string, kept map[string]bool) {
	dirs := make([]string, 0)
	_ = filepath.WalkDir(top, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	// A group sorts after its parent, whose path prefixes its own.
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		err := os.Remove(dir)
		switch {
		case err == nil:
			if c.log != nil {
				c.log.Debugf("removed pg %s", dir)
			}
		case errors.Is(err, os.ErrNotExist):
		default:
			kept[filepath.Base(dir)] = true
			if c.log != nil {
				c.log.Debugf("keep pg %s: %s", dir, err)
			}
		}
	}
}

// revertProperties has the systemd instance d names, the system's when it is
// nil, forget the properties set on the slice of the group and on the slices
// nested in it, but the slices whose group is kept.
//
// The slices are found from the drop-in directories systemctl set-property
// writes, so the ones whose group is already gone are found too. Every slice
// nested in the one of the group is named after it, followed by a dash: the
// name of an object escapes its own dashes.
func (c Config) revertProperties(d *Delegation, dirs []string, kept map[string]bool) error {
	units := c.slicesWithDropIns(dirs, kept)
	if len(units) == 0 || !hasSystemd() {
		return nil
	}
	sort.Strings(units)
	path := c.ID
	if d != nil {
		path = d.Root + c.ID
	}
	if err := runSystemctl(d, append([]string{"revert"}, units...)); err != nil {
		return fmt.Errorf("pg %s: %w", path, err)
	}
	if c.log != nil {
		c.log.Infof("reverted the systemd properties of %s", strings.Join(units, " "))
	}
	return nil
}

// slicesWithDropIns are the slice of the group and the slices nested in it
// having a drop-in directory in dirs, but the kept ones.
func (c Config) slicesWithDropIns(dirs []string, kept map[string]bool) []string {
	units := make([]string, 0)
	unit := c.unit()
	if unit == "" {
		return units
	}
	prefix := strings.TrimSuffix(unit, ".slice") + "-"
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".d")
			if !ok || !strings.HasSuffix(name, ".slice") {
				continue
			}
			if name != unit && !strings.HasPrefix(name, prefix) {
				continue
			}
			if kept[name] || slices.Contains(units, name) {
				continue
			}
			units = append(units, name)
		}
	}
	return units
}

// forgetV1Properties has a systemd older than 236 forget the cappings set
// on the slice of the group and on the slices nested in it, but the slices
// whose group is kept.
//
// Such a systemd, as the 219 of rhel7, has no revert, and keeps the runtime
// properties in files named after them among the runtime units, where a
// later one keeps them in a control directory revertProperties reads. Each
// slice holding one of these files is given every property unset, so the
// unit systemd may still hold in memory is uncapped, then the files are
// removed, so the unit loaded again is not capped by them. A later systemd
// has no such file, and nothing is done. The slices made to forget are
// returned.
func (c Config) forgetV1Properties(kept map[string]bool) ([]string, error) {
	forgotten := make([]string, 0)
	if !hasSystemd() {
		return forgotten, nil
	}
	var errs error
	units := c.slicesWithDropIns([]string{legacyRuntimeDir}, kept)
	sort.Strings(units)
	for _, unit := range units {
		dir := filepath.Join(legacyRuntimeDir, unit+".d")
		props := make([]v1Property, 0)
		for _, p := range v1PropertyKinds {
			if _, err := os.Stat(filepath.Join(dir, p.dropIn)); err == nil {
				p.value = v1PropertyUnset
				props = append(props, p)
			}
		}
		if len(props) == 0 {
			continue
		}
		forgotten = append(forgotten, unit)
		if err := c.setUnitV1Properties(unit, props, false); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg %s: %w", c.ID, err))
		}
		for _, p := range props {
			if err := os.Remove(filepath.Join(dir, p.dropIn)); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = errors.Join(errs, fmt.Errorf("pg %s: %w", c.ID, err))
			}
		}
		// The directory is left when it holds the file of another
		// property, which is not one of the cappings.
		_ = os.Remove(dir)
		if c.log != nil {
			c.log.Infof("unset the systemd properties of %s", unit)
		}
	}
	return forgotten, errs
}

// stopSlices stops the slices units of the systemd of the system, the
// nested ones first.
//
// A slice a container was started in, or one om started for a systemd 219
// to take its properties, stays an active unit when its last process ends,
// and systemd makes the groups of an active unit again, in the hierarchies
// of the controllers it accounts, when its properties are reverted: the
// groups om had just removed were back, empty and uncapped. They are the
// slices whose group is gone from every hierarchy, the one systemd keeps its
// units in included, so no unit is left in one to be stopped with it. A
// slice systemd has not loaded is not running.
//
// Each stop is waited for, see stopUnitJob, all of them within
// stopSlicesTimeout. A stop not seen done by then is reported and is no
// error, the job being queued: the slices left are then stopped without
// waiting, as systemd no longer answers in time.
func (c Config) stopSlices(units []string) error {
	if len(units) == 0 || !hasSystemd() {
		return nil
	}
	// A nested slice is named after the one holding it, so it is the
	// longer of the two.
	sort.Slice(units, func(i, j int) bool {
		if len(units[i]) != len(units[j]) {
			return len(units[i]) > len(units[j])
		}
		return units[i] < units[j]
	})
	ctx, cancel := context.WithTimeout(context.Background(), stopSlicesTimeout)
	defer cancel()
	var errs error
	wait := true
	stopped := make([]string, 0, len(units))
	for _, unit := range units {
		err := stopUnit(ctx, unit, wait)
		switch {
		case err == nil:
		case isNoSuchUnit(err):
			continue
		case errors.Is(err, errUnitStopPending):
			if c.log != nil {
				c.log.Warnf("the stop of the systemd slice %s was not seen done in %s: a start coming before it may lose caps", unit, stopSlicesTimeout)
			}
			// The time of the calls is spent too: the stops left
			// are given the one they had before the waits.
			unwaited, cancelUnwaited := context.WithTimeout(context.Background(), stopSlicesTimeout)
			defer cancelUnwaited()
			ctx, wait = unwaited, false
		default:
			errs = errors.Join(errs, fmt.Errorf("pg %s: stop %s: %w", c.ID, unit, err))
			continue
		}
		stopped = append(stopped, unit)
	}
	if len(stopped) > 0 && c.log != nil {
		c.log.Infof("stopped the systemd slices %s", strings.Join(stopped, " "))
	}
	return errs
}

// stopSlicesTimeout is the time systemd is given to stop the slices of a
// group removed. It is replaced by the tests.
var stopSlicesTimeout = ipFilterTimeout

// userDelegations are the trees systemd delegates to the users whose systemd
// instance runs, as podman places a rootless container in.
func userDelegations() []Delegation {
	base := filepath.Join(UnifiedPath(), "user.slice")
	users, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	l := make([]Delegation, 0)
	for _, u := range users {
		if !u.IsDir() || !strings.HasPrefix(u.Name(), "user-") || !strings.HasSuffix(u.Name(), ".slice") {
			continue
		}
		services, err := os.ReadDir(filepath.Join(base, u.Name()))
		if err != nil {
			continue
		}
		for _, s := range services {
			var uid int
			if _, err := fmt.Sscanf(s.Name(), "user@%d.service", &uid); err != nil {
				continue
			}
			if s.Name() != fmt.Sprintf("user@%d.service", uid) || !s.IsDir() {
				continue
			}
			info, err := os.Stat(filepath.Join(base, u.Name(), s.Name()))
			if err != nil {
				continue
			}
			gid := 0
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				gid = int(st.Gid)
			}
			l = append(l, Delegation{
				Root: "/user.slice/" + u.Name() + "/" + s.Name(),
				UID:  uid,
				GID:  gid,
			})
		}
	}
	return l
}
