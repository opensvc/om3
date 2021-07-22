package object

import (
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
)

// WithConfigFile sets a non-standard configuration location.
func WithConfigFile(s string) funcopt.O {
	return funcopt.F(func(t interface{}) error {
		base := t.(*Base)
		base.configFile = s
		return nil
	})
}

// WithVolatile makes sure not data is ever written by the object.
func WithVolatile(s bool) funcopt.O {
	return funcopt.F(func(t interface{}) error {
		base := t.(*Base)
		base.volatile = s
		return nil
	})
}

// NewFromPath allocates a new kinded object
func NewFromPath(p path.T, opts ...funcopt.O) interface{} {
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
		return nil
	}
}

// NewBaserFromPath returns a Baser interface from an object path
func NewBaserFromPath(p path.T, opts ...funcopt.O) Baser {
	return NewFromPath(p, opts...).(Baser)
}

// NewConfigurerFromPath returns a Configurer interface from an object path
func NewConfigurerFromPath(p path.T, opts ...funcopt.O) Configurer {
	return NewFromPath(p, opts...).(Configurer)
}

// NewActorFromPath returns a Actor interface from an object path
func NewActorFromPath(p path.T, opts ...funcopt.O) Actor {
	return NewFromPath(p, opts...).(Actor)
}
