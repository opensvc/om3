package object

import (
	"crypto/subtle"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/key"
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

func (t *usr) KeywordLookup(k key.T, sectionType string) *keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}

// GrantsFromUsernameAndPassword returns grants for username if username and password match existing usr object
func (_ *UsrDB) GrantsFromUsernameAndPassword(username, password string) ([]string, error) {
	usrPath := naming.Path{Name: username, Namespace: naming.NsSys, Kind: naming.KindUsr}
	user, err := NewUsr(usrPath, WithVolatile(true))
	if err != nil {
		return nil, err
	}
	storedPassword, err := user.DecodeKey("password")
	if err != nil {
		return nil, fmt.Errorf("read password from %s: %w", usrPath, err)
	}
	// The comparison is constant time so that it is one less thing to
	// think about. The reply this leads to says only that the
	// credentials were refused, which is what actually keeps the two
	// failures below apart from a caller.
	if subtle.ConstantTimeCompare(storedPassword, []byte(password)) != 1 {
		return nil, fmt.Errorf("wrong password")
	}
	return user.Config().GetStrings(key.T{Section: "DEFAULT", Option: "grant"}), nil
}

// UsrFromUXUser returns the name and the grants of the user of the system
// namespace whose uxuser keyword names login or uid, the local account
// connected to the user socket. A user naming neither is no local account,
// whatever its name. It returns an empty name when no user names them, and
// refuses several users naming them, a configuration to fix rather than
// grants to merge.
func (_ *UsrDB) UsrFromUXUser(login string, uid uint32) (string, []string, error) {
	paths, err := naming.InstalledPaths()
	if err != nil {
		return "", nil, err
	}
	uidStr := strconv.FormatUint(uint64(uid), 10)
	var (
		name   string
		grants []string
	)
	for _, p := range paths {
		if p.Kind != naming.KindUsr || p.Namespace != naming.NsSys {
			continue
		}
		user, err := NewUsr(p, WithVolatile(true))
		if err != nil {
			continue
		}
		cfg := user.Config()
		for _, s := range cfg.GetStrings(key.T{Section: "DEFAULT", Option: "uxuser"}) {
			if s != uidStr && (login == "" || s != login) {
				continue
			}
			if name != "" && name != p.Name {
				return "", nil, fmt.Errorf("the users %s and %s both name the local account %s (uid %d) in uxuser", name, p.Name, login, uid)
			}
			name = p.Name
			grants = cfg.GetStrings(key.T{Section: "DEFAULT", Option: "grant"})
		}
	}
	return name, grants, nil
}

// UXGrantsFromCgroup returns the name and the grants of the container
// resource whose process group holds the cgroup cg, the one of a process of
// a sidecar connected to the user socket: the uxgrant keyword of the
// resource. The name is "<path>:<rid>". It returns an empty name when no
// container resource of the node grants anything to that cgroup.
//
// The configurations are read first, and an object is loaded only when one
// of its container sections sets uxgrant, to compute the process group of
// that resource.
func (_ *UsrDB) UXGrantsFromCgroup(cg string) (string, []string, error) {
	if cg == "" || cg == "/" {
		return "", nil, nil
	}
	paths, err := naming.InstalledPaths()
	if err != nil {
		return "", nil, err
	}
	for _, p := range paths {
		if p.Kind != naming.KindSvc && p.Kind != naming.KindVol {
			continue
		}
		c, err := NewCore(p, WithVolatile(true))
		if err != nil {
			continue
		}
		cfg := c.Config()
		rids := make([]string, 0)
		for _, section := range cfg.SectionStrings() {
			if strings.HasPrefix(section, "container#") && len(cfg.GetStrings(key.T{Section: section, Option: "uxgrant"})) > 0 {
				rids = append(rids, section)
			}
		}
		if len(rids) == 0 {
			continue
		}
		a, err := NewActor(p, WithVolatile(true))
		if err != nil {
			continue
		}
		// The resources are configured, and their process groups
		// set, on the first listing.
		for _, r := range a.Resources() {
			rid := r.RID()
			if !slices.Contains(rids, rid) {
				continue
			}
			// A rootless container runs under the subtree of its
			// user, not where its process group id says.
			pgPath := r.GetPGID()
			if p, ok := r.(resource.PGPather); ok {
				pgPath = p.PGPath()
			}
			if pgPath == "" || (cg != pgPath && !strings.HasPrefix(cg, pgPath+"/")) {
				continue
			}
			return p.String() + ":" + rid, cfg.GetStrings(key.T{Section: rid, Option: "uxgrant"}), nil
		}
	}
	return "", nil, nil
}

// GrantsFromUsername returns grants for username if username existing usr object
func (_ *UsrDB) GrantsFromUsername(username string) ([]string, error) {
	usrPath := naming.Path{Name: username, Namespace: naming.NsSys, Kind: naming.KindUsr}
	if !usrPath.Exists() {
		return nil, fmt.Errorf("username '%s' does not exist", username)
	}
	user, err := NewUsr(usrPath, WithVolatile(true))
	if err != nil {
		return nil, err
	}
	return user.Config().GetStrings(key.T{Section: "DEFAULT", Option: "grant"}), nil
}
