package object

import (
	"fmt"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/key"
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

	// UsrDB implements UserGrants to authenticate user and get its grants
	UsrDB struct{}
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

// UserGrants returns grants for username if username and password match existing usr object
func (_ *UsrDB) UserGrants(username, password string) ([]string, error) {
	usrPath := path.T{Name: username, Namespace: "system", Kind: kind.Usr}
	user, err := NewUsr(usrPath, WithVolatile(true))
	if err != nil {
		return nil, err
	}
	storedPassword, err := user.DecodeKey("password")
	if err != nil {
		return nil, fmt.Errorf("read password from %s: %w", usrPath, err)
	}
	if string(storedPassword) != password {
		return nil, fmt.Errorf("wrong password")
	}
	return user.Config().GetStrings(key.T{Section: "DEFAULT", Option: "grant"}), nil
}
