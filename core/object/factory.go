package object

import (
	"fmt"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/xerrors"
)

// WithConfigFile sets a non-standard configuration location.
func WithConfigFile(s string) funcopt.O {
	return funcopt.F(func(t interface{}) error {
		o := t.(*core)
		o.configFile = s
		return nil
	})
}

// WithConfigData sets configuration overriding what is installed in the config file
// Useful for testing volatile services.
func WithConfigData(b any) funcopt.O {
	return funcopt.F(func(t interface{}) error {
		o := t.(*core)
		o.configData = b
		return nil
	})
}

// WithVolatile makes sure not data is ever written by the object.
func WithVolatile(s bool) funcopt.O {
	return funcopt.F(func(t any) error {
		o := t.(volatiler)
		o.SetVolatile(s)
		return nil
	})
}

func NewList(paths path.L, opts ...funcopt.O) ([]interface{}, error) {
	var errs error
	l := make([]interface{}, 0)
	for _, p := range paths {
		if obj, err := New(p, opts...); err != nil {
			xerrors.Append(errs, err)
		} else {
			l = append(l, obj)
		}
	}
	return l, errs
}

func toPathType(id any) (path.T, error) {
	var p path.T
	switch i := id.(type) {
	case string:
		if parsed, err := path.Parse(i); err != nil {
			return p, err
		} else {
			p = parsed
		}
		return p, nil
	case path.T:
		p = i
		return p, nil
	default:
		return p, errors.Errorf("unsupported object path type: %#v", i)
	}
}

// New allocates a new kinded object
func New(id any, opts ...funcopt.O) (any, error) {
	var p path.T
	if parsed, err := toPathType(id); err != nil {
		return nil, err
	} else {
		p = parsed
	}
	switch p.Kind {
	case kind.Svc:
		return NewSvc(p, opts...)
	case kind.Vol:
		return NewVol(p, opts...)
	case kind.Cfg:
		return NewCfg(p, opts...)
	case kind.Sec:
		return NewSec(p, opts...)
	case kind.Usr:
		return NewUsr(p, opts...)
	case kind.Ccfg:
		return NewCcfg(p, opts...)
	default:
		return nil, fmt.Errorf("unsupported kind: %s", p.Kind)
	}
}

// NewCore returns a Core interface from an object path
func NewCore(p any, opts ...funcopt.O) (Core, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Core), nil
	}
}

// NewConfigurer returns a Configurer interface from an object path
func NewConfigurer(p any, opts ...funcopt.O) (Configurer, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Configurer), nil
	}
}

// NewActor returns a Actor interface from an object path
func NewActor(p any, opts ...funcopt.O) (Actor, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Actor), nil
	}
}

// NewKeystore returns a Keystore interface from an object path
func NewKeystore(p any, opts ...funcopt.O) (Keystore, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Keystore), nil
	}
}
