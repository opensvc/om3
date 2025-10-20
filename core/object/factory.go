package object

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

var ErrWrongType = errors.New("wrong type provided for interface")

// WithConfigFile sets a non-standard configuration location.
func WithConfigFile(s string) funcopt.O {
	return funcopt.F(func(t any) error {
		if o, ok := t.(*core); ok {
			o.configFile = s
		} else if o, ok := t.(*Node); ok {
			o.configFile = s
		} else {
			return fmt.Errorf("WithConfigFile() is not supported on %v", t)
		}
		return nil
	})
}

// WithConfigFile sets a non-standard configuration location.
func WithClusterConfigFile(s string) funcopt.O {
	return funcopt.F(func(t any) error {
		if o, ok := t.(*Node); ok {
			o.clusterConfigFile = s
		} else {
			return fmt.Errorf("WithClusterConfigFile() is not supported on %v", t)
		}
		return nil
	})
}

// WithConfigData sets configuration file (string) or content ([]byte)
// overriding what is installed in the config file.
// Useful for testing volatile services.
func WithConfigData(b any) funcopt.O {
	return funcopt.F(func(t any) error {
		if o, ok := t.(*core); ok {
			o.configData = b
		} else if o, ok := t.(*Node); ok {
			o.configData = b
		} else {
			return fmt.Errorf("WithConfigData() is not supported on %v", t)
		}
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
	case naming.KindNscfg:
		return NewNscfg(p, opts...)
	default:
		return nil, fmt.Errorf("unsupported kind: %s", p.Kind)
	}
}

// NewCore returns a Core interface from an object path
func NewCore(p naming.Path, opts ...funcopt.O) (Core, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else if i, ok := o.(Core); ok {
		return i, nil
	} else {
		return nil, ErrWrongType
	}
}

// NewConfigurer returns a Configurer interface from an object path
func NewConfigurer(p naming.Path, opts ...funcopt.O) (Configurer, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else if i, ok := o.(Configurer); ok {
		return i, nil
	} else {
		return nil, ErrWrongType
	}
}

// NewActor returns a Actor interface from an object path
func NewActor(p naming.Path, opts ...funcopt.O) (Actor, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else if i, ok := o.(Actor); ok {
		return i, nil
	} else {
		return nil, ErrWrongType
	}
}

// NewDataStore returns a DataStore interface from an object path
func NewDataStore(p naming.Path, opts ...funcopt.O) (DataStore, error) {
	if o, err := New(p, opts...); err != nil {
		return nil, err
	} else if i, ok := o.(DataStore); ok {
		return i, nil
	} else {
		return nil, ErrWrongType
	}
}
