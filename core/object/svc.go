package object

import "opensvc.com/opensvc/core/path"

type (
	//
	// Svc is the svc-kind object.
	//
	// These objects contain front facing resources like app and containers.
	//
	Svc struct {
		Base
	}
)

// NewSvc allocates a svc kind object.
func NewSvc(p path.T) *Svc {
	s := &Svc{}
	s.Base.init(p)
	return s
}
