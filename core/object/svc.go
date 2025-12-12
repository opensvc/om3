package object

import (
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	svc struct {
		actor
	}

	//
	// Svc is the svc-kind object.
	//
	// These objects contain front facing resources like app and containers.
	//
	Svc interface {
		Actor
	}
)

// NewSvc allocates a svc kind object.
func NewSvc(path naming.Path, opts ...funcopt.O) (*svc, error) {
	s := &svc{}
	s.path = path
	s.path.Kind = naming.KindSvc
	err := s.init(s, path, opts...)
	return s, err
}

func (t *svc) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}
