package object

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

// WithConfigFile sets a non-standard configuration location.
func WithConfigFile(s string) funcopt.O {
	return funcopt.F(func(t any) error {
		o := t.(*core)
		o.configFile = s
		return nil
	})
}

// WithConfigData sets configuration overriding what is installed in the config file
// Useful for testing volatile services.
func WithConfigData(b any) funcopt.O {
	return funcopt.F(func(t any) error {
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

// WithLogger let the factory user decide what kind of logging he wants
func WithLogger(s *plog.Logger) funcopt.O {
	return funcopt.F(func(t any) error {
		switch o := t.(type) {
		case *core:
			o.log = s
		case *Node:
			o.log = s
		}
		return nil
	})
}

func NewList(paths naming.Paths, opts ...funcopt.O) ([]any, error) {
	var errs error
	l := make([]any, 0)
	for _, p := range paths {
		if obj, err := New(p, opts...); err != nil {
			errors.Join(errs, err)
		} else {
			l = append(l, obj)
		}
	}
	return l, errs
}

// New allocates a new kinded object
func New(p naming.Path, opts ...funcopt.O) (any, error) {
	switch p.Kind {
	case naming.KindSvc:
		return NewSvc(p, opts...)
	case naming.KindVol:
		return NewVol(p, opts...)
	case naming.KindCfg:
		return NewCfg(p, opts...)
	case naming.KindSec:
		return NewSec(p, opts...)
	case naming.KindUsr:
		return NewUsr(p, opts...)
	case naming.KindCcfg:
		return newCcfg(p, opts...)
	default:
		return nil, fmt.Errorf("unsupported kind: %s", p.Kind)
	}
}

// NewCore returns a Core interface from an object path
func NewCore(p naming.Path, opts ...funcopt.O) (Core, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Core), nil
	}
}

// NewConfigurer returns a Configurer interface from an object path
func NewConfigurer(p naming.Path, opts ...funcopt.O) (Configurer, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Configurer), nil
	}
}

// NewActor returns a Actor interface from an object path
func NewActor(p naming.Path, opts ...funcopt.O) (Actor, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Actor), nil
	}
}

// NewKeystore returns a Keystore interface from an object path
func NewKeystore(p naming.Path, opts ...funcopt.O) (Keystore, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Keystore), nil
	}
}
