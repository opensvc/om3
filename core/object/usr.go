package object

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	//
	// Usr is the usr-kind object.
	//
	// These objects contain a opensvc api user grants and credentials.
	// They are required for basic, session and x509 api access, but not
	// for OpenID access (where grants are embedded in the trusted token)
	//
	Usr struct {
		Base
	}
)

// NewUsr allocates a usr kind object.
func NewUsr(p path.T, opts ...funcopt.O) *Usr {
	s := &Usr{}
	s.Base.init(p, opts...)
	s.Config().RegisterPostCommit(s.postCommit)
	return s
}
