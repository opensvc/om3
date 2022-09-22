package object

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/key"
)

type (
	usr struct {
		sec
	}

	//
	// Usr is the usr-kind object.
	//
	// These objects contain a opensvc api user grants and credentials.
	// They are required for basic, session and x509 api access, but not
	// for OpenID access (where grants are embedded in the trusted token)
	//
	Usr interface {
		Sec
	}
)

// NewUsr allocates a usr kind object.
func NewUsr(p any, opts ...funcopt.O) (*usr, error) {
	s := &usr{}
	s.customEncode = secEncode
	s.customDecode = secDecode
	if err := s.init(s, p, opts...); err != nil {
		return s, err
	}
	s.Config().RegisterPostCommit(s.postCommit)
	return s, nil
}

func (t usr) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}
