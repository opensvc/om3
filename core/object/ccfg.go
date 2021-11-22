package object

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/key"
)

type (
	//
	// Ccfg is the clusterwide configuration store.
	//
	// The content is the same as node.conf, and is overriden by
	// the definition found in node.conf.
	//
	Ccfg struct {
		Base
	}
)

var ccfgKeywordStore = keywords.Store(commonKeywords)

// NewCcfg allocates a ccfg kind object.
func NewCcfg(p path.T, opts ...funcopt.O) (*Ccfg, error) {
	s := &Ccfg{}
	err := s.Base.init(s, p, opts...)
	return s, err
}

func (t Ccfg) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	switch k.Section {
	case "data", "env", "labels":
		return keywords.Keyword{
			Option:   "*", // trick IsZero()
			Scopable: true,
			Required: false,
		}
	}
	return keywordLookup(ccfgKeywordStore, k, t.Path.Kind, sectionType)
}
