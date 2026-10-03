// Package vpath is a helper package easing the expansion of a virtual path like
// vol1/etc/nginx.conf to a host path like
// /srv/svc1data.ns1.vol.clu1/etc/nginx.conf
//
// A path is written in one of three forms:
//
//	/etc/nginx.conf            a path of the node
//	vol1/etc/nginx.conf        a path in the vol object vol1, of the namespace
//	volume#1:/etc/nginx.conf   a path in a resource of the object: a volume,
//	                           or a filesystem, under its mount point
//
// A vol name is a hostname, which holds no '#', so a reference before the
// ':' holding one is a resource id, and the forms never mean one another.
package vpath

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/loop"
)

type (
	// ErrAccess is the error of a path in a vol or a resource that is not
	// available here.
	ErrAccess struct {
		Path  naming.Path
		RID   string
		Avail status.T
	}

	// Resolver finds the resources of the object a path is written in.
	Resolver interface {
		ResourceByID(string) resource.Driver
	}

	// Target is what a path resolves to.
	Target struct {
		// HostPath is the path on the node.
		HostPath string

		// Head is the mount point of the vol or the resource the path is
		// in, and empty for a path of the node.
		Head string

		// Vol is the vol the path is in, nil for a path of the node, or of
		// a resource that is no volume.
		Vol object.Vol
	}

	header interface {
		Head() string
	}

	voler interface {
		Volume() (object.Vol, error)
	}
)

func (t ErrAccess) Error() string {
	if t.RID != "" {
		return fmt.Sprintf("resource is not accessible: %s is avail %s", t.RID, t.Avail)
	}
	return fmt.Sprintf("vol is not accessible: %s is avail %s", t.Path, t.Avail)
}

// IsResourceRef says whether the reference before the ':' of a path is the
// id of a resource of the object, as volume#1 in volume#1:/etc/nginx. A path
// of the node starts with a '/', and a vol name holds no '#'.
func IsResourceRef(s string) bool {
	return s != "" && !strings.HasPrefix(s, "/") && !strings.Contains(s, "/") && strings.Contains(s, "#")
}

// ResolverOf returns the resolver of the resources of an object, nil when it
// has none.
func ResolverOf(o any) Resolver {
	r, _ := o.(Resolver)
	return r
}

// Resolve expands a path to the path of the node it names, and says the
// mount point and the vol it is in.
//
// A path in a resource of the object needs the resolver of the object, the
// resources being its own. The resource must hold a mount point, as a volume
// or a filesystem does, and be available here.
func Resolve(ctx context.Context, s string, namespace string, resolver Resolver) (Target, error) {
	if strings.HasPrefix(s, "/") {
		return Target{HostPath: s}, nil
	}
	first, rel, isResource := strings.Cut(s, ":")
	if !isResource || !IsResourceRef(first) {
		if ref, _, _ := strings.Cut(s, "/"); strings.Contains(ref, "#") {
			return Target{}, fmt.Errorf("%s: a path in a resource is written <rid>:/<path>, as volume#1:/etc/nginx", s)
		}
		hostPath, vol, err := volHostPath(ctx, s, namespace)
		if err != nil || vol == nil {
			return Target{HostPath: hostPath}, err
		}
		return Target{HostPath: hostPath, Head: vol.Head(), Vol: vol}, nil
	}
	if !strings.HasPrefix(rel, "/") {
		return Target{}, fmt.Errorf("%s: a path in a resource is written <rid>:/<path>, as %s:/%s", s, first, rel)
	}
	if _, err := resourceid.Parse(first); err != nil {
		return Target{}, fmt.Errorf("%s: %s is not a resource id: %w", s, first, err)
	}
	if resolver == nil {
		return Target{}, fmt.Errorf("%s: the resource %s is not known here", s, first)
	}
	r := resolver.ResourceByID(first)
	if r == nil {
		return Target{}, fmt.Errorf("%s: no resource %s", s, first)
	}
	h, ok := r.(header)
	if !ok {
		return Target{}, fmt.Errorf("%s: the resource %s holds no mount point: it is no volume nor filesystem", s, first)
	}
	switch avail := r.Status(ctx); avail {
	case status.Up, status.NotApplicable, status.StandbyUp:
	default:
		return Target{}, ErrAccess{RID: first, Avail: avail}
	}
	head := h.Head()
	if head == "" {
		return Target{}, fmt.Errorf("%s: the resource %s has no mount point here", s, first)
	}
	// The path is cleaned as a path from the mount point, so it never
	// leads above it.
	t := Target{HostPath: filepath.Join(head, filepath.Clean(rel)), Head: head}
	if v, ok := r.(voler); ok {
		if vol, err := v.Volume(); err == nil {
			t.Vol = vol
		}
	}
	return t, nil
}

// HostPathAndVol expand a volume-relative path to a host full path. It returns
// the host full path and the associated object volume if defined.
//
// Example:
//
// INPUT        VOL     host path            COMMENT
// /path        nil     /path                host full path
// myvol/path   myvol   /srv/myvol/path      vol head relative path
func HostPathAndVol(ctx context.Context, s string, namespace string) (hostPath string, vol object.Vol, err error) {
	t, err := Resolve(ctx, s, namespace, nil)
	return t.HostPath, t.Vol, err
}

// volHostPath expands a path in a vol, or of the node, to a path of the node.
func volHostPath(ctx context.Context, s string, namespace string) (hostPath string, vol object.Vol, err error) {
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
		err = fmt.Errorf("%w: %s", xerrors.ObjectNotFound, vol.Path())
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
		err = ErrAccess{
			Path:  volPath,
			Avail: volStatus.Avail,
		}
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

// ResolveHostPath expands a path, in any of its forms, to a path of the node.
func ResolveHostPath(ctx context.Context, s string, namespace string, resolver Resolver) (string, error) {
	t, err := Resolve(ctx, s, namespace, resolver)
	return t.HostPath, err
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
		if lo, err := loop.New().FileGet(ctx, s); err != nil {
			return "", err
		} else if lo != nil {
			return lo.Name, nil
		} else {
			// loopback not active
			return "", nil
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
		err = ErrAccess{
			Path:  volPath,
			Avail: st.Avail,
		}
		return s, err
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
