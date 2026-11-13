//go:build linux

package pg

import (
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
// properties again in the group it makes for the slice the next time.
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
	errs := c.revertProperties(nil, systemControlDirs, kept)
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
	unit := c.unit()
	if unit == "" {
		return nil
	}
	prefix := strings.TrimSuffix(unit, ".slice") + "-"
	units := make([]string, 0)
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
