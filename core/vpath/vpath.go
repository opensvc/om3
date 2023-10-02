// Package vpath is a helper package easing the expansion of a virtual path like
// vol1/etc/nginx.conf to a host path like
// /srv/svc1data.ns1.vol.clu1/etc/nginx.conf
package vpath

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/loop"
)

var (
	ErrAccess = errors.New("vol is not accessible")
)

// HostPath expand a volume-relative path to a host full path.
//
// Example:
//
// INPUT        VOL     OUPUT            COMMENT
// /path                /path            host full path
// myvol/path   myvol   /srv/myvol/path  vol head relative path
func HostPath(s string, namespace string) (string, error) {
	var volRelativeSourcePath string
	l := strings.SplitN(s, "/", 2)
	if len(l[0]) == 0 {
		return s, nil
	}
	if len(l) == 2 {
		volRelativeSourcePath = l[1]
	}
	volPath := naming.Path{
		Name:      l[0],
		Namespace: namespace,
		Kind:      naming.KindVol,
	}
	vol, err := object.NewVol(volPath)
	if err != nil {
		return s, err
	}
	if !vol.Path().Exists() {
		return s, fmt.Errorf("%s does not exist", vol.Path())
	}
	st, err := vol.Status(context.Background())
	if err != nil {
		return s, err
	}
	switch st.Avail {
	case status.Up, status.NotApplicable, status.StandbyUp:
	default:
		return s, fmt.Errorf("%w: %s(%s)", ErrAccess, volPath, st.Avail)
	}
	return vol.Head() + "/" + volRelativeSourcePath, nil
}

// HostPaths applies the HostPath function to each path of the input list
func HostPaths(l []string, namespace string) ([]string, error) {
	for i, s := range l {
		if s2, err := HostPath(s, namespace); err != nil {
			return l, err
		} else {
			l[i] = s2
		}
	}
	return l, nil
}

// HostDevpath returns host device path for a volume
// translation rules:
// INPUT        VOL     OUPUT       COMMENT
// /path                /dev/sda1   loop dev
// /dev/sda1            /dev/sda1   host full path
// myvol        myvol   /dev/sda1   vol dev path in host
func HostDevpath(s string, namespace string) (string, error) {
	if strings.HasPrefix(s, "/dev/") {
		return s, nil
	}
	if v, err := file.ExistsAndRegular(s); err != nil {
		return s, err
	} else if v {
		if lo, err := loop.New().FileGet(s); err != nil {
			return "", err
		} else {
			return lo.Name, nil
		}
	}
	// volume device
	volPath := naming.Path{
		Name:      s,
		Namespace: namespace,
		Kind:      naming.KindVol,
	}
	vol, err := object.NewVol(volPath)
	if err != nil {
		return s, err
	}
	st, err := vol.Status(context.Background())
	if err != nil {
		return s, err
	}
	switch st.Avail {
	case status.Up, status.NotApplicable, status.StandbyUp:
	default:
		return s, fmt.Errorf("%w: %s(%s)", ErrAccess, volPath, st.Avail)
	}
	dev := vol.Device()
	if dev == nil {
		return s, fmt.Errorf("%s is not a device-capable vol", s)
	}
	return dev.Path(), nil
}

// HostDevpaths applies the HostDevpath function to each path of the input list
func HostDevpaths(l []string, namespace string) ([]string, error) {
	for i, s := range l {
		if s2, err := HostDevpath(s, namespace); err != nil {
			return l, err
		} else {
			l[i] = s2
		}
	}
	return l, nil
}
