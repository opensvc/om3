// +build linux

package main

import (
	"strings"

	"opensvc.com/opensvc/core/check"
	"opensvc.com/opensvc/util/df"
)

func (t Type) parseDF() ([]*check.Result, error) {
	r := make([]*check.Result, 0)
	data, err := df.Do()
	if err != nil {
		return nil, err
	}
	for _, e := range data {
		// discard bind mounts: we get metric from the source anyway
		if strings.HasPrefix(e.Device, "/") && !strings.HasPrefix(e.Device, "/dev") && !strings.HasPrefix(e.Device, "//") {
			continue
		}
		if e.Device == "overlay" {
			continue
		}
		if e.Device == "overlay2" {
			continue
		}
		if e.Device == "aufs" {
			continue
		}
		if strings.HasPrefix(e.MountPoint, "/Volumes") {
			continue
		}
		if strings.HasPrefix(e.MountPoint, "/media/") {
			continue
		}
		if strings.HasPrefix(e.MountPoint, "/run") {
			continue
		}
		if strings.HasPrefix(e.MountPoint, "/sys/") {
			continue
		}
		if strings.HasPrefix(e.MountPoint, "/shm") {
			continue
		}
		if strings.HasPrefix(e.MountPoint, "/snap/") {
			continue
		}
		if strings.Contains(e.MountPoint, "/overlay2/") {
			continue
		}
		if strings.Contains(e.MountPoint, "/snapd/") {
			continue
		}
		if strings.Contains(e.MountPoint, "/graph/") {
			continue
		}
		if strings.Contains(e.MountPoint, "/aufs/mnt/") {
			continue
		}
		if strings.Contains(e.Device, "osvc_sync_") {
			// do not report osvc sync snapshots fs usage
			continue
		}
		path := t.ObjectPath(e.MountPoint)
		r = append(r, &check.Result{
			Instance: e.MountPoint,
			Value:    e.UsedPercent,
			Path:     path,
			Unit:     "%",
		})
		r = append(r, &check.Result{
			Instance: e.MountPoint + ".free",
			Value:    e.Free,
			Path:     path,
			Unit:     "kb",
		})
		r = append(r, &check.Result{
			Instance: e.MountPoint + ".size",
			Value:    e.Total,
			Path:     path,
			Unit:     "kb",
		})
	}
	return r, nil
}
