package object

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/key"
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
	err := s.Base.init(s, p, opts...)
	return s, err
}

func (t Svc) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(keywordStore, k, t.Path.Kind, sectionType)
}
