package object

import (
	"fmt"

	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
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
func WithConfigData(b []byte) funcopt.O {
	return funcopt.F(func(t interface{}) error {
		o := t.(*core)
		o.configData = b
		return nil
	})
}

// WithVolatile makes sure not data is ever written by the object.
func WithVolatile(s bool) funcopt.O {
	return funcopt.F(func(t interface{}) error {
		o := t.(*core)
		o.volatile = s
		return nil
	})
}

// NewFromPath allocates a new kinded object
func NewFromPath(p path.T, opts ...funcopt.O) (interface{}, error) {
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

// NewBaserFromPath returns a Baser interface from an object path
func NewBaserFromPath(p path.T, opts ...funcopt.O) (Core, error) {
	if o, err := NewFromPath(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Core), nil
	}
}

// NewConfigurerFromPath returns a Configurer interface from an object path
func NewConfigurerFromPath(p path.T, opts ...funcopt.O) (Configurer, error) {
	if o, err := NewFromPath(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Configurer), nil
	}
}

// NewActorFromPath returns a Actor interface from an object path
func NewActorFromPath(p path.T, opts ...funcopt.O) (Actor, error) {
	if o, err := NewFromPath(p, opts...); err != nil {
		return nil, err
	} else {
		return o.(Actor), nil
	}
}
