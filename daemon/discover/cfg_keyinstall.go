package discover

import (
	"context"
	"os"
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/runner"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"
)

// fsDirectoryType is the type a directory resource says in its status.
var fsDirectoryType = driver.NewID(driver.GroupFS, "directory").String()

// keyInstallTimeout bounds an install, which signals resources, as a
// container, with timeouts of their own.
const keyInstallTimeout = 2 * time.Minute

var (
	keyInstallCmdPath = func() string {
		p, err := os.Executable()
		if err != nil {
			return "/bin/false"
		}
		return p
	}()

	// runKeyInstall runs an install as the daemon runs the actions of its
	// orchestrations: in one of the node.max_parallel slots, by priority.
	runKeyInstall = runner.Run
)

// nodeMaxParallel returns the node.max_parallel of the node, the most
// installs the key installer submits at once, or the least the daemon
// allows before the node configuration is known.
func nodeMaxParallel(nodename string) int {
	if c := node.ConfigData.GetByNode(nodename); c != nil && c.MaxParallel > 0 {
		return c.MaxParallel
	}
	return object.MinNodeMaxParallel
}

// keyReceiverRIDs returns the volume and fs resources of an instance status,
// and whether one of them may receive keys now: one up, or a directory that
// exists, whose status does not apply, and which keys are installed in
// whether the instance runs or not.
//
// Only a directory: other resources say their status does not apply when
// they hold nothing to install in, as a filesystem with no device or mount
// point, a resource failing to configure, or a driver the agent does not
// have.
func keyReceiverRIDs(st *instance.Status) (rids []string, up bool) {
	if st == nil {
		return nil, false
	}
	for rid, rst := range st.Resources {
		id, err := resourceid.Parse(rid)
		if err != nil {
			continue
		}
		switch id.DriverGroup() {
		case driver.GroupVolume, driver.GroupFS:
		default:
			continue
		}
		rids = append(rids, rid)
		switch {
		case rst.Status.Is(status.Up, status.StandbyUp):
			up = true
		case rst.Status == status.NotApplicable && rst.Type == fsDirectoryType:
			up = true
		}
	}
	return rids, up
}

// mayInstallKeysOf says whether the local instance of the svc may install
// keys of the datastore p, its status st and its configuration cfg as the
// daemon holds them.
//
// The instance needs a volume or fs resource up, or a directory. Then its
// configuration says: the datastores its resources use, the ones named by
// patterns included, as the node evaluates it, a datastore not existing yet
// included. A configuration not known yet, or older than the file of the
// configuration, as when a svc changes its install lines, says yes: the
// install command run reads the lines themselves, and changes nothing where
// they name another datastore. A template of a volume reading a key of p at
// its last render says yes too.
func mayInstallKeysOf(p naming.Path, st *instance.Status, cfg *instance.Config, configMtime func() time.Time, templateReadsFrom func(rid string) bool) bool {
	rids, up := keyReceiverRIDs(st)
	if !up {
		return false
	}
	if cfg == nil || cfg.ActorConfig == nil {
		return true
	}
	if !configMtime().Equal(cfg.UpdatedAt) {
		return true
	}
	if cfg.UsesObject(p) {
		return true
	}
	for _, rid := range rids {
		if templateReadsFrom(rid) {
			return true
		}
	}
	return false
}

// keyRefreshPriority says whether a local instance may install keys of the
// datastore p, and the priority to refresh them at: the one of the most
// urgent of these instances.
func (t *Manager) keyRefreshPriority(p naming.Path) (priority.T, bool) {
	var (
		found  bool
		prio   priority.T
		loaded bool
		store  object.DataStore
	)
	allow := func(namespace string) bool {
		if namespace == p.Namespace {
			return true
		}
		if !loaded {
			// Loaded only for an instance of another namespace, to
			// read the namespaces the datastore is shared with.
			loaded = true
			var err error
			if store, err = object.NewDataStore(p, object.WithVolatile(true)); err != nil {
				t.objectLogger(p).Warnf("cfg: refresh the installed keys of %s: %s", p, err)
				store = nil
			}
		}
		return store != nil && store.Allow(namespace)
	}
	for _, svc := range instance.StatusData.GetPathsByNode(t.localhost) {
		if svc.Kind != naming.KindSvc {
			continue
		}
		st := instance.StatusData.GetByPathAndNode(svc, t.localhost)
		cfg := instance.ConfigData.GetByPathAndNode(svc, t.localhost)
		configMtime := func() time.Time { return file.ModTime(svc.ConfigFile()) }
		templateReadsFrom := func(rid string) bool { return datarecv.TemplateReadsFrom(svc, rid, p) }
		if !mayInstallKeysOf(p, st, cfg, configMtime, templateReadsFrom) {
			continue
		}
		if !allow(svc.Namespace) {
			continue
		}
		svcPrio := priority.T(priority.Default)
		if cfg != nil {
			svcPrio = cfg.Priority
		}
		if !found || svcPrio < prio {
			prio = svcPrio
		}
		found = true
	}
	return prio, found
}

// onLocalConfigUpdated refreshes the keys of the datastores a resource of a
// new configuration of a local svc instance starts using, as a resource
// enabled again, or given an install line: nothing installs them otherwise
// until the resource starts, or the datastore changes.
//
// It is said resource by resource: a resource enabled again starts using a
// datastore another resource of the object uses already, which the uses of
// the object do not say.
//
// The first configuration of an instance seen, at the start of the daemon or
// of the object, only records what it uses: a start, or a provision,
// installs what it uses.
func (t *Manager) onLocalConfigUpdated(p naming.Path, cfg instance.Config) {
	if p.Kind != naming.KindSvc {
		return
	}
	uses := make(map[string][]string)
	if cfg.ActorConfig != nil {
		for rid, rcfg := range cfg.Resources {
			if len(rcfg.Uses) > 0 {
				uses[rid] = slices.Clone(rcfg.Uses)
			}
		}
	}
	former, known := t.localUses[p]
	t.localUses[p] = uses
	if !known {
		return
	}
	var started []string
	for rid, l := range uses {
		for _, use := range l {
			if !slices.Contains(former[rid], use) && !slices.Contains(started, use) {
				started = append(started, use)
			}
		}
	}
	slices.Sort(started)
	for _, use := range started {
		for _, store := range localDataStoresOf(use) {
			t.objectLogger(p).Infof("cfg: %s starts using %s: refresh its keys", p, store)
			t.keyInstaller.request(store)
		}
	}
}

// localDataStoresOf returns the local datastores an entry of the uses of an
// instance names: the datastore, when it exists, or the ones its pattern
// matches. A vol is no datastore.
func localDataStoresOf(use string) []naming.Path {
	isLocalDataStore := func(p naming.Path) bool {
		return slices.Contains(naming.KindDataStore, p.Kind) && p.Exists()
	}
	if !naming.IsPathPattern(use) {
		p, err := naming.ParsePath(use)
		if err != nil || !isLocalDataStore(p) {
			return nil
		}
		return []naming.Path{p}
	}
	pattern, err := naming.ParsePathPatternRel(use, naming.NsRoot)
	if err != nil {
		return nil
	}
	paths, err := naming.InstalledPaths()
	if err != nil {
		return nil
	}
	var l []naming.Path
	for _, p := range paths {
		if pattern.MatchPath(p) && isLocalDataStore(p) {
			l = append(l, p)
		}
	}
	return l
}

// onDataStoreConfigFetched refreshes the keys of the datastore p installed in
// the local volumes.
func (t *Manager) onDataStoreConfigFetched(p naming.Path) {
	if !slices.Contains(naming.KindDataStore, p.Kind) {
		return
	}
	t.keyInstaller.request(p)
}

// refreshInstalledKeys runs the install of the keys of the datastore p in
// the local volumes, when a local instance may install them, in a runner
// slot. It calls starting when the slot is acquired.
func (t *Manager) refreshInstalledKeys(ctx context.Context, p naming.Path, starting func()) {
	log := t.objectLogger(p)
	prio, ok := t.keyRefreshPriority(p)
	if !ok {
		log.Tracef("cfg: no local instance may install keys of %s: no refresh", p)
		return
	}
	_ = runKeyInstall(prio, p.String()+": key install", func() error {
		if ctx.Err() != nil {
			return nil
		}
		starting()
		t.execKeyInstall(ctx, p)
		return nil
	})
}

func (t *Manager) execKeyInstall(ctx context.Context, p naming.Path) {
	log := t.objectLogger(p)
	ctx, cancel := context.WithTimeout(ctx, keyInstallTimeout)
	defer cancel()
	args := []string{p.String(), "key", "install"}
	log.Infof("cfg: refresh the keys of %s installed in the local volumes: -> exec %s %s", p, keyInstallCmdPath, args)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(keyInstallCmdPath),
		command.WithArgs(args),
		command.WithLogger(log),
	)
	if err := cmd.Run(); err != nil {
		log.Warnf("cfg: refresh the keys of %s installed in the local volumes: %s", p, err)
	}
}
