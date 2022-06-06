package object

import (
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/loop"
)

var (
	ErrAccess = errors.New("vol is not accessible")
)

//
// translation rules:
// INPUT        VOL     OUPUT            COMMENT
// /path                /path            host full path
// myvol/path   myvol   /srv/myvol/path  vol head relative path
//
func Realpath(s string, namespace string) (string, error) {
	l := strings.SplitN(s, "/", 2)
	if len(l[0]) == 0 {
		return s, nil
	}
	volPath := path.T{
		Name:      l[0],
		Namespace: namespace,
		Kind:      kind.Vol,
	}
	vol, err := NewVol(volPath)
	if err != nil {
		return s, err
	}
	st, err := vol.Status(OptsStatus{})
	if err != nil {
		return s, err
	}
	switch st.Avail {
	case status.Up, status.NotApplicable, status.StandbyUp:
	default:
		return s, errors.Wrapf(ErrAccess, "%s(%s)", volPath, st.Avail)
	}
	return vol.Head() + "/" + l[1], nil
}

func Realpaths(l []string, namespace string) ([]string, error) {
	for i, s := range l {
		if s2, err := Realpath(s, namespace); err != nil {
			return l, err
		} else {
			l[i] = s2
		}
	}
	return l, nil
}

//
// translation rules:
// INPUT        VOL     OUPUT       COMMENT
// /path                /dev/sda1   loop dev
// /dev/sda1            /dev/sda1   host full path
// myvol        myvol   /dev/sda1   vol dev path in host
//
func Realdevpath(s string, namespace string) (string, error) {
	if strings.HasPrefix(s, "/dev/") {
		return s, nil
	} else if file.ExistsAndRegular(s) {
		if lo, err := loop.New().FileGet(s); err != nil {
			return "", err
		} else {
			return lo.Name, nil
		}
		return s, nil
	} else {
		// volume device
		volPath := path.T{
			Name:      s,
			Namespace: namespace,
			Kind:      kind.Vol,
		}
		vol, err := NewVol(volPath)
		if err != nil {
			return s, err
		}
		st, err := vol.Status(OptsStatus{})
		if err != nil {
			return s, err
		}
		switch st.Avail {
		case status.Up, status.NotApplicable, status.StandbyUp:
		default:
			return s, errors.Wrapf(ErrAccess, "%s(%s)", volPath, st.Avail)
		}
		dev := vol.Device()
		if dev == nil {
			return s, errors.Errorf("%s is not a device-capable vol", s)
		}
		return dev.Path(), nil
	}
}

func Realdevpaths(l []string, namespace string) ([]string, error) {
	for i, s := range l {
		if s2, err := Realdevpath(s, namespace); err != nil {
			return l, err
		} else {
			l[i] = s2
		}
	}
	return l, nil
}
