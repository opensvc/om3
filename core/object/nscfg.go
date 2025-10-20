package object

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/key"
)

type (
	nscfg struct {
		core
	}

	Nscfg interface {
		Core
	}
)

func NewNscfg(path naming.Path, opts ...funcopt.O) (*nscfg, error) {
	s := &nscfg{}
	s.path = path
	s.path.Kind = naming.KindNscfg
	if err := s.init(s, path, opts...); err != nil {
		return s, err
	}
	s.Config().RegisterPostCommit(s.postCommit)
	return s, nil
}

func (t *nscfg) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}
