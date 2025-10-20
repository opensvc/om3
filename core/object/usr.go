package object

import (
	"fmt"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
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
func NewUsr(path naming.Path, opts ...funcopt.O) (*usr, error) {
	s := &usr{}
	s.path = path
	s.path.Kind = naming.KindUsr
	if err := s.init(s, path, opts...); err != nil {
		return s, err
	}
	ed, err := GetSecEncryptDecrypter()
	if err != nil {
		return s, err
	}
	s.encodeDecoder = &secEncodeDecode{
		encryptDecrypter: ed,
	}
	s.Config().RegisterPostCommit(s.postCommit)
	return s, nil
}

func (t *usr) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}

// GrantsFromUsernameAndPassword returns grants for username if username and password match existing usr object
func (_ *UsrDB) GrantsFromUsernameAndPassword(username, password string) ([]string, error) {
	usrPath := naming.Path{Name: username, Namespace: naming.NamespaceSystem, Kind: naming.KindUsr}
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

// GrantsFromUsername returns grants for username if username existing usr object
func (_ *UsrDB) GrantsFromUsername(username string) ([]string, error) {
	usrPath := naming.Path{Name: username, Namespace: naming.NamespaceSystem, Kind: naming.KindUsr}
	if !usrPath.Exists() {
		return nil, fmt.Errorf("username '%s' does not exist", username)
	}
	user, err := NewUsr(usrPath, WithVolatile(true))
	if err != nil {
		return nil, err
	}
	return user.Config().GetStrings(key.T{Section: "DEFAULT", Option: "grant"}), nil
}
