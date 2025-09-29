package device

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"syscall"

	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

type (
	T struct {
		path string
		log  *plog.Logger
	}
	L []T
)

const (
	ModeBlock uint = syscall.S_IFBLK
	ModeChar  uint = syscall.S_IFCHR
)

func New(path string, opts ...funcopt.O) T {
	t := T{
		path: path,
	}
	_ = funcopt.Apply(&t, opts...)
	return t
}

func WithLogger(log *plog.Logger) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

func (t T) String() string {
	return t.path
}

func (t T) Path() string {
	return t.path
}

func (t T) RemoveHolders() error {
	return RemoveHolders(t)
}

func RemoveHolders(head T) error {
	holders, err := head.Holders()
	if err != nil {
		return err
	}
	for _, dev := range holders {
		if err := RemoveHolders(dev); err != nil {
			return err
		}
		if err := dev.Remove(); err != nil {
			return err
		}
	}
	return nil
}

// MarshalJSON marshals the data as a quoted json string
func (t T) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(t.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to value
func (t *T) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t = New(j)
	return nil
}

func (t T) SCSIPaths() (L, error) {
	isSCSI, err := t.IsSCSI()
	if err != nil {
		return L{}, err
	}
	if isSCSI {
		return L{t}, nil
	}
	isMpath, err := t.IsMultipath()
	if err != nil {
		return L{}, err
	}
	if !isMpath {
		return L{}, nil
	}
	return t.Slaves()
}

func (t L) Contains(dev T) (bool, error) {
	ref, err := dev.MajorMinorStr()
	if err != nil {
		return false, err
	}
	for _, dev = range t {
		s, err := dev.MajorMinorStr()
		if err != nil {
			return false, err
		}
		if s == ref {
			return true, nil
		}
	}
	return false, nil
}

func (t L) SCSIPaths() (L, error) {
	var errs error
	l := make(L, 0)
	for _, dev := range t {
		if paths, err := dev.SCSIPaths(); err != nil {
			errs = errors.Join(errs, err)
		} else {
			l = append(l, paths...)
		}
	}
	return l, errs
}

func recurse(l L, dev T) (L, error) {
	slaves, err := dev.Slaves()
	if err != nil {
		return l, err
	}
	for _, slave := range slaves {
		l = slices.DeleteFunc(l, func(e T) bool {
			return e.path == slave.path
		})
		l, err = recurse(l, slave)
		if err != nil {
			return l, err
		}
	}
	return l, nil
}

func (t L) HolderEndpoints() (L, error) {
	var err error
	l := slices.Clone(t)
	for _, dev := range t {
		if l, err = recurse(l, dev); err != nil {
			return l, err
		}
	}
	return l, nil
}
