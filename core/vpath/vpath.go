// Package vpath is a helper package easing the expansion of a virtual path like
// vol1/etc/nginx.conf to a host path like
// /srv/svc1data.ns1.vol.clu1/etc/nginx.conf
package vpath

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/loop"
)

var (
	ErrAccess = errors.New("vol is not accessible")
)

// HostPathAndVol expand a volume-relative path to a host full path. It returns
// the host full path and the associated object volume if defined.
//
// Example:
//
// INPUT        VOL     host path            COMMENT
// /path        nil     /path                host full path
// myvol/path   myvol   /srv/myvol/path      vol head relative path
func HostPathAndVol(ctx context.Context, s string, namespace string) (hostPath string, vol object.Vol, err error) {
	var volRelativeSourcePath string
	l := strings.SplitN(s, "/", 2)
	if len(l[0]) == 0 {
		hostPath = s
		return
	}
	if len(l) == 2 {
		volRelativeSourcePath = l[1]
	}
	volPath := naming.Path{
		Name:      l[0],
		Namespace: namespace,
		Kind:      naming.KindVol,
	}
	vol, err = object.NewVol(volPath)
	if err != nil {
		return
	}
	if !vol.Path().Exists() {
		err = fmt.Errorf("%s does not exist", vol.Path())
		return
	}

	volStatus, err1 := vol.Status(ctx)
	if err1 != nil {
		err = err1
		return
	}
	switch volStatus.Avail {
	case status.Up, status.NotApplicable, status.StandbyUp:
	default:
		err = fmt.Errorf("%w: %s(%s)", ErrAccess, volPath, volStatus.Avail)
		return
	}
	hostPath = vol.Head() + "/" + volRelativeSourcePath
	return
}

// HostPath expand a volume-relative path to a host full path.
//
// Example:
//
// INPUT        VOL     OUTPUT           COMMENT
// /path                /path            host full path
// myvol/path   myvol   /srv/myvol/path  vol head relative path
func HostPath(ctx context.Context, s string, namespace string) (string, error) {
	hostPath, _, err := HostPathAndVol(ctx, s, namespace)
	return hostPath, err
}

// HostPaths applies the HostPath function to each path of the input list
func HostPaths(ctx context.Context, l []string, namespace string) ([]string, error) {
	for i, s := range l {
		if s2, err := HostPath(ctx, s, namespace); err != nil {
			return l, err
		} else {
			l[i] = s2
		}
	}
	return l, nil
}

// HostDevpath returns host device path for a volume
// translation rules:
// INPUT        VOL     OUTPUT      COMMENT
// /path                /dev/sda1   loop dev
// /dev/sda1            /dev/sda1   host full path
// myvol        myvol   /dev/sda1   vol dev path in host
func HostDevpath(ctx context.Context, s string, namespace string) (string, error) {
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
	st, err := vol.Status(ctx)
	if err != nil {
		return s, err
	}
	switch st.Avail {
	case status.Up, status.NotApplicable, status.StandbyUp:
	default:
		return s, fmt.Errorf("%w: %s(%s)", ErrAccess, volPath, st.Avail)
	}
	dev := vol.ExposedDevice(ctx)
	if dev == nil {
		return s, fmt.Errorf("%s is not a device-capable vol", s)
	}
	return dev.Path(), nil
}

// HostDevpaths applies the HostDevpath function to each path of the input list
func HostDevpaths(ctx context.Context, l []string, namespace string) ([]string, error) {
	for i, s := range l {
		if s2, err := HostDevpath(ctx, s, namespace); err != nil {
			return l, err
		} else {
			l[i] = s2
		}
	}
	return l, nil
}
