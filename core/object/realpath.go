package object

import (
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
)

var (
	ErrAccess = errors.New("vol is not accessible")
)

//
// translation rules:
// "/path"      => []string{"",      "path"} => host full path
// "myvol/path" => []string{"myvol", "path"} => vol head relative path
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
