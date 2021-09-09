package object

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
)

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
func NewSvc(p path.T, opts ...funcopt.O) (*Svc, error) {
	s := &Svc{}
	err := s.Base.init(p, opts...)
	return s, err
}
