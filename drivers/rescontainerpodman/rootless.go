package rescontainerpodman

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/usergroup"
)

var _ resource.IDMapper = (*T)(nil)

// rootlessUser is the unprivileged user a rootless container is run by.
type rootlessUser struct {
	Name string
	UID  uint32
	GID  uint32
	Home string
}

var (
	// subuidFile and subgidFile hold the subordinate ids podman maps the
	// users of a rootless container to.
	subuidFile = "/etc/subuid"
	subgidFile = "/etc/subgid"

	// runtimeDirRoot holds the runtime directory of every user, made by
	// systemd-logind while the systemd instance of the user runs.
	runtimeDirRoot = "/run/user"
)

// rootlessUser returns the user the container is run by, nil when it is run
// by root.
func (t *T) rootlessUser() (*rootlessUser, error) {
	if t.RootlessUser == "" {
		return nil, nil
	}
	u, err := usergroup.LookupUser(t.RootlessUser)
	if err != nil {
		return nil, fmt.Errorf("rootless_user %s: %w", t.RootlessUser, err)
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("rootless_user %s: uid %s: %w", t.RootlessUser, u.Uid, err)
	}
	if uid == 0 {
		return nil, fmt.Errorf("rootless_user %s is root: a rootless container is run by an unprivileged user", t.RootlessUser)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("rootless_user %s: gid %s: %w", t.RootlessUser, u.Gid, err)
	}
	if t.RootlessGroup != "" {
		if g, err := usergroup.GIDFromString(t.RootlessGroup); err != nil {
			return nil, fmt.Errorf("rootless_group %s: %w", t.RootlessGroup, err)
		} else {
			gid = uint64(g)
		}
	}
	return &rootlessUser{
		Name: u.Username,
		UID:  uint32(uid),
		GID:  uint32(gid),
		Home: u.HomeDir,
	}, nil
}

// runtimeDir is the runtime directory of the user, where podman keeps what it
// needs between two commands.
func (u rootlessUser) runtimeDir() string {
	return filepath.Join(runtimeDirRoot, strconv.FormatUint(uint64(u.UID), 10))
}

// check says what the node lacks for podman to run a container as the user.
//
// Each is a setup of the node an operator makes once, and none is one om can
// guess the intent of, so they are refused by name rather than worked around:
// without them podman fails later, at the boot of a node, with an error
// naming neither.
func (u rootlessUser) check() error {
	var errs []error
	if _, err := os.Stat(u.runtimeDir()); err != nil {
		errs = append(errs, fmt.Errorf("the systemd instance of %s is not running, %s does not exist: run 'loginctl enable-linger %s' so it runs without a login session", u.Name, u.runtimeDir(), u.Name))
	}
	for _, path := range []string{subuidFile, subgidFile} {
		if ok, err := hasSubordinateIDs(path, u.Name, u.UID); err != nil {
			errs = append(errs, err)
		} else if !ok {
			errs = append(errs, fmt.Errorf("%s has no subordinate ids in %s: podman cannot map the users of a rootless container", u.Name, path))
		}
	}
	return errors.Join(errs...)
}

// hasSubordinateIDs reports whether path grants the user a range of
// subordinate ids, by name or by uid, as shadow-utils reads it.
func hasSubordinateIDs(path, name string, uid uint32) (bool, error) {
	ranges, err := subordinateRanges(path, name, uid)
	return len(ranges) > 0, err
}

// idRange is a range of subordinate ids: count ids from start.
type idRange struct {
	start uint64
	count uint64
}

// subordinateRanges returns the ranges of subordinate ids path grants the
// user, by name or by uid, in the order of the file, which is the order
// podman hands them to newuidmap and newgidmap in.
func subordinateRanges(path, name string, uid uint32) ([]idRange, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	id := strconv.FormatUint(uint64(uid), 10)
	ranges := make([]idRange, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		l := strings.Split(strings.TrimSpace(scanner.Text()), ":")
		if len(l) != 3 {
			continue
		}
		if l[0] != name && l[0] != id {
			continue
		}
		start, err := strconv.ParseUint(l[1], 10, 32)
		if err != nil {
			continue
		}
		count, err := strconv.ParseUint(l[2], 10, 32)
		if err != nil || count == 0 {
			continue
		}
		ranges = append(ranges, idRange{start: start, count: count})
	}
	return ranges, scanner.Err()
}

// hostID returns the host id the id of a rootless container runs as, in the
// mapping podman makes by default: the root of the container is the user,
// and the ids from 1 are the subordinate ids of the user, one range after the
// other.
func hostID(id uint32, owner uint32, path, name string, uid uint32) (uint32, error) {
	if id == 0 {
		return owner, nil
	}
	ranges, err := subordinateRanges(path, name, uid)
	if err != nil {
		return 0, err
	}
	offset := uint64(id) - 1
	var total uint64
	for _, r := range ranges {
		if offset < r.count {
			return uint32(r.start + offset), nil
		}
		offset -= r.count
		total += r.count
	}
	return 0, fmt.Errorf("id %d is not mapped: %s has %d subordinate ids in %s, mapped to the ids 1 to %d of the container", id, name, total, path, total)
}

// HostUID implements resource.IDMapper: the host uid the uid id of the
// container runs as.
func (t *T) HostUID(id uint32) (uint32, error) {
	u, err := t.hostIDUser()
	if err != nil {
		return 0, err
	} else if u == nil {
		return id, nil
	}
	return hostID(id, u.UID, subuidFile, u.Name, u.UID)
}

// HostGID implements resource.IDMapper: the host gid the gid id of the
// container runs as. The root group of the container is the group podman
// runs as, which rootless_group sets.
func (t *T) HostGID(id uint32) (uint32, error) {
	u, err := t.hostIDUser()
	if err != nil {
		return 0, err
	} else if u == nil {
		return id, nil
	}
	return hostID(id, u.GID, subgidFile, u.Name, u.UID)
}

// hostIDUser returns the user whose mapping the container runs in, nil for a
// container running its ids as themselves.
//
// Only the mappings om knows are answered: the one podman makes by default,
// and none. A userns keyword asks podman for another, which podman makes when
// the container starts, and an id computed here would be a guess an install
// could chown files by.
func (t *T) hostIDUser() (*rootlessUser, error) {
	u, err := t.rootlessUser()
	if err != nil {
		return nil, err
	}
	switch t.UserNS {
	case "", "host":
	default:
		return nil, fmt.Errorf("the host ids of a container run with userns=%s are not known before it runs", t.UserNS)
	}
	return u, nil
}

// credential is who the podman commands run as, and the environment they
// find the store and the runtime directory of the user in.
//
// The XDG base directories are set to where they default: the agent may run
// with its own, and podman reading root's would look for the user's
// containers in a store the user cannot open.
func (u rootlessUser) credential() *rescontainerocibase.Credential {
	runtimeDir := u.runtimeDir()
	return &rescontainerocibase.Credential{
		UID:  u.UID,
		GID:  u.GID,
		Home: u.Home,
		Env: []string{
			"HOME=" + u.Home,
			"USER=" + u.Name,
			"LOGNAME=" + u.Name,
			"XDG_RUNTIME_DIR=" + runtimeDir,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=" + filepath.Join(runtimeDir, "bus"),
			"XDG_CONFIG_HOME=" + filepath.Join(u.Home, ".config"),
			"XDG_DATA_HOME=" + filepath.Join(u.Home, ".local", "share"),
			"XDG_CACHE_HOME=" + filepath.Join(u.Home, ".cache"),
		},
	}
}

// delegation is the subtree of the cgroup hierarchy systemd delegates to the
// user: the one their systemd instance makes the groups of their units in,
// and so the one podman places a rootless container in.
func (u rootlessUser) delegation() pg.Delegation {
	return pg.Delegation{
		Root: fmt.Sprintf("/user.slice/user-%d.slice/user@%d.service", u.UID, u.UID),
		UID:  int(u.UID),
		GID:  int(u.GID),
	}
}

// resolvConfDir makes the directory the resolver of the container is written
// in, under the runtime directory of the user.
//
// The var dir of the resource is under one only root can enter, and podman
// mounts the file as the user. The runtime directory is the user's, is
// emptied when their systemd instance stops, and the file is written again on
// every start, which is when it is read.
func (t *T) resolvConfDir(u *rootlessUser) (string, error) {
	rel, err := filepath.Rel(rawconfig.Paths.Var, t.VarDir())
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = filepath.Join(t.Path.String(), t.RID())
	}
	base := u.runtimeDir()
	dir := base
	for _, name := range append([]string{"opensvc"}, strings.Split(rel, string(filepath.Separator))...) {
		dir = filepath.Join(dir, name)
		if err := os.Mkdir(dir, 0755); err != nil && !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("resolv.conf dir: %w", err)
		}
		if err := os.Chown(dir, int(u.UID), int(u.GID)); err != nil {
			return "", fmt.Errorf("resolv.conf dir: %w", err)
		}
	}
	return dir, nil
}

// ApplyPG caps the container where podman places it.
//
// A rootless container is placed by the systemd instance of its user, under
// the subtree systemd delegates to them, so the groups om caps are made
// there, owned by the user for their systemd to make the group of the
// container inside. The groups of the object and its namespace are made
// there too, with their cappings: a group is capped by the ones it is nested
// in, and those are only above it if they are made above it.
//
// The cappings are written by om in files the user is not given, so they hold
// against the user as they do against the container.
func (t *T) ApplyPG(ctx context.Context) error {
	u, err := t.rootlessUser()
	if err != nil {
		return err
	} else if u == nil {
		return t.BT.ApplyPG(ctx)
	}
	if err := u.check(); err != nil {
		return err
	}
	cfg := t.GetPG()
	if cfg == nil {
		return nil
	}
	mgr := pg.FromContext(ctx)
	if mgr == nil {
		return nil
	}
	t.registerDelegatedPG(mgr, cfg, u)
	return mgr.ApplyConfigs()
}

// ResetPG lifts the capping of the groups the container is placed in.
func (t *T) ResetPG(ctx context.Context) error {
	u, err := t.rootlessUser()
	if err != nil {
		return err
	} else if u == nil {
		return t.BT.ResetPG(ctx)
	}
	cfg := t.GetPG()
	if cfg == nil {
		return nil
	}
	mgr := pg.FromContext(ctx)
	if mgr == nil {
		return nil
	}
	t.registerDelegatedPG(mgr, cfg, u)
	return mgr.ResetConfigs()
}

// registerDelegatedPG makes the groups of the container in the tree of the
// user, nested in copies of the groups of its object and subset.
//
// The groups of the namespace and the node are not copied: a copy in each
// user tree would give each tree the whole budget again, which caps nothing
// the namespace consumes. The namespace consumption is accounted for by its
// claims instead, which count the caps of its objects wherever they run.
func (t *T) registerDelegatedPG(mgr *pg.Mgr, cfg *pg.Config, u *rootlessUser) {
	d := u.delegation()
	for _, c := range mgr.Ancestors(cfg.ID) {
		if !t.isObjectGroup(c.ID) {
			continue
		}
		mgr.Register(c.Delegated(d))
	}
	mgr.Register(cfg.Delegated(d).WithLogger(t.Log()))
}

// isObjectGroup is true for a group of the object of the container or nested
// in it, and false for the groups of its namespace and of the node, which the
// object groups are nested in: /opensvc.slice for the node and the root
// namespace, /opensvc.slice/opensvc-<namespace>.slice for another namespace.
func (t *T) isObjectGroup(id string) bool {
	depth := strings.Count(strings.TrimSuffix(id, "/"), "/")
	if t.Path.Namespace == naming.NsRoot {
		return depth > 1
	}
	return depth > 2
}

// Start refuses to run a rootless container on a node that lacks what podman
// needs to, naming what it lacks.
func (t *T) Start(ctx context.Context) error {
	if u, err := t.rootlessUser(); err != nil {
		return err
	} else if u != nil {
		if err := u.check(); err != nil {
			return err
		}
	}
	return t.BT.Start(ctx)
}

// Status warns about what the node lacks for the container to run rootless:
// it is what an operator reading why the container is down needs to see.
func (t *T) Status(ctx context.Context) status.T {
	for _, err := range t.RootlessIssues() {
		t.StatusLog().Warn("%s", err)
	}
	return t.BT.Status(ctx)
}

// RootlessIssues returns what keeps the container from running rootless on
// this node, one error per issue, none for a container run by root.
//
// It is exported for the podman task, which runs its command in a container
// of this driver and says the same in its own status.
func (t *T) RootlessIssues() []error {
	u, err := t.rootlessUser()
	if err != nil {
		return []error{err}
	} else if u == nil {
		return nil
	}
	if err := u.check(); err != nil {
		return flatten(err)
	}
	return nil
}

// flatten returns the errors an errors.Join made, one per line of the status.
func flatten(err error) []error {
	if l, ok := err.(interface{ Unwrap() []error }); ok {
		return l.Unwrap()
	}
	return []error{err}
}
