// Package rootless is the accounts a namespace may run rootless containers as.
//
// A rootless container runs its podman commands as the unix account its
// rootless_user keyword names, and as the group rootless_group names, or the
// primary group of the account. The account owns the container, its image
// store and its files, so an account is a boundary: whoever runs a container
// as it reaches everything else that account owns on the node, the rootless
// containers of other objects included.
//
// Which accounts are the namespace's to use is not the namespace's to decide,
// so the namespace configuration lists them, and only the squatter writes it:
//
//	[DEFAULT]
//	rootless_users = web-ns1
//	rootless_groups = web-ns1
//
// A namespace listing none has no account to run rootless containers as. An
// allowed user allows its primary group. The root account and the root group
// are never allowed, whatever the lists say.
package rootless

import (
	"fmt"
	"os/user"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	keyUsers  = key.New("DEFAULT", "rootless_users")
	keyGroups = key.New("DEFAULT", "rootless_groups")
)

// Allowed is what a namespace lists, as written.
type Allowed struct {
	Namespace string
	Users     []string
	Groups    []string
}

// Load reads what a namespace allows from its configuration on this node, and
// allows nothing for a namespace whose configuration this node does not hold.
//
// It is read from the node and never over the api, as a claim is: a namespace
// configuration is on every node.
func Load(namespace string) (Allowed, error) {
	a := Allowed{Namespace: namespace}
	p := naming.Path{Namespace: namespace, Kind: naming.KindNscfg, Name: "namespace"}
	configFile := p.ConfigFile()
	if !file.Exists(configFile) {
		return a, nil
	}
	// The configuration file path is both the write target and the source to
	// read, so it is passed twice.
	cfg, err := xconfig.NewObject(configFile, configFile)
	if err != nil {
		return a, err
	}
	a.Users = strings.Fields(cfg.Get(keyUsers))
	a.Groups = strings.Fields(cfg.Get(keyGroups))
	return a, nil
}

// Check says why a namespace may not run a container as the user and the
// group given, and nil when it may. An empty user is a rootful container,
// which is not an account of the namespace.
//
// The ids are compared, not the names: an account allowed by name is the same
// account named by its uid, and a name resolving to another uid on another
// node is another account there.
func (a Allowed) Check(userName, groupName string) error {
	if userName == "" {
		return nil
	}
	u, err := lookupUser(userName)
	if err != nil {
		// The node judging a write need not be one of the nodes of the
		// object, and need not have its accounts. What it can judge is the
		// name: an account the namespace lists by that name is the one the
		// squatter allowed, and the nodes running the container resolve it.
		if isRootGroup(groupName) {
			return fmt.Errorf("rootless_group %s is the root group", groupName)
		}
		if slices.Contains(a.Users, userName) && (groupName == "" || slices.Contains(a.Groups, groupName)) {
			return nil
		}
		return fmt.Errorf("rootless_user %s: %w, and the %s namespace does not list it by that name: it allows %s", userName, err, a.Namespace, a.says(a.Users))
	}
	if u.Uid == "0" {
		return fmt.Errorf("rootless_user %s is root", userName)
	}
	gid := u.Gid
	if groupName != "" {
		g, err := lookupGroup(groupName)
		if err != nil {
			return fmt.Errorf("rootless_group %s: %w", groupName, err)
		}
		gid = g.Gid
	}
	if gid == "0" {
		return fmt.Errorf("rootless_group %s is the root group", groupName)
	}
	uids, gids := a.ids()
	if !slices.Contains(uids, u.Uid) {
		return fmt.Errorf("the %s namespace does not allow rootless_user %s (uid %s): it allows %s", a.Namespace, userName, u.Uid, a.says(a.Users))
	}
	if !slices.Contains(gids, gid) {
		name := groupName
		if name == "" {
			name = "gid " + gid
		}
		return fmt.Errorf("the %s namespace does not allow rootless_group %s: it allows the primary groups of its users and %s", a.Namespace, name, a.says(a.Groups))
	}
	return nil
}

// ids resolves what the namespace lists to the uids and the gids they are on
// this node. An entry this node does not resolve allows nothing here.
func (a Allowed) ids() ([]string, []string) {
	uids := make([]string, 0, len(a.Users))
	gids := make([]string, 0, len(a.Users)+len(a.Groups))
	for _, s := range a.Users {
		if u, err := lookupUser(s); err == nil && u.Uid != "0" {
			uids = append(uids, u.Uid)
			if u.Gid != "0" {
				gids = append(gids, u.Gid)
			}
		}
	}
	for _, s := range a.Groups {
		if g, err := lookupGroup(s); err == nil && g.Gid != "0" {
			gids = append(gids, g.Gid)
		}
	}
	return uids, gids
}

func (a Allowed) says(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	return strings.Join(l, " ")
}

// SharedWith returns the other namespaces allowing one of the users this one
// allows, by the uid they share.
//
// Two namespaces running containers as the same account reach each other's
// containers, images and files, which is allowed, the squatter deciding it,
// and said.
func (a Allowed) SharedWith() (map[string][]string, error) {
	uids, _ := a.ids()
	shared := make(map[string][]string)
	if len(uids) == 0 {
		return shared, nil
	}
	namespaces, err := namespaces()
	if err != nil {
		return nil, err
	}
	for _, namespace := range namespaces {
		if namespace == a.Namespace {
			continue
		}
		other, err := Load(namespace)
		if err != nil {
			continue
		}
		otherUIDs, _ := other.ids()
		for _, uid := range uids {
			if slices.Contains(otherUIDs, uid) {
				shared[uid] = append(shared[uid], namespace)
			}
		}
	}
	for uid := range shared {
		sort.Strings(shared[uid])
	}
	return shared, nil
}

// DescribeShared says the namespaces sharing the accounts of a namespace, for
// a warning.
func DescribeShared(shared map[string][]string) string {
	l := make([]string, 0, len(shared))
	for uid, namespaces := range shared {
		name := "uid " + uid
		if u, err := user.LookupId(uid); err == nil {
			name = u.Username
		}
		l = append(l, fmt.Sprintf("%s is also allowed in %s", name, strings.Join(namespaces, ", ")))
	}
	sort.Strings(l)
	return strings.Join(l, "; ")
}

// namespaces returns the namespaces this node holds a configuration of.
func namespaces() ([]string, error) {
	l, err := naming.InstalledPaths()
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool)
	for _, p := range l {
		if p.Kind == naming.KindNscfg {
			m[p.Namespace] = true
		}
	}
	names := make([]string, 0, len(m))
	for namespace := range m {
		names = append(names, namespace)
	}
	sort.Strings(names)
	return names, nil
}

// isRootGroup reports whether a group names the root group, by gid, by the
// name root, or by a name this node resolves to gid 0. A node that does not
// resolve the name cannot say more, and the node running the container refuses
// the root group when it does.
func isRootGroup(s string) bool {
	switch s {
	case "0", "root":
		return true
	}
	if g, err := lookupGroup(s); err == nil && g.Gid == "0" {
		return true
	}
	return false
}

func lookupUser(s string) (*user.User, error) {
	if _, err := strconv.Atoi(s); err == nil {
		return user.LookupId(s)
	}
	return user.Lookup(s)
}

func lookupGroup(s string) (*user.Group, error) {
	if _, err := strconv.Atoi(s); err == nil {
		return user.LookupGroupId(s)
	}
	return user.LookupGroup(s)
}
