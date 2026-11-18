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

// upVolumeRIDs returns the volume and fs resources of an instance status,
// and whether one of them is up: the instance holds volumes keys are
// installed in.
func upVolumeRIDs(st *instance.Status) (rids []string, up bool) {
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
		if rst.Status.Is(status.Up, status.StandbyUp) {
			up = true
		}
	}
	return rids, up
}

// mayInstallKeysOf says whether the local instance of the svc may install
// keys of the datastore p, its status st and its configuration cfg as the
// daemon holds them.
//
// The instance needs a volume or fs resource up. Then its configuration
// says: the datastores its resources use, the ones named by patterns
// included, as the node evaluates it, a datastore not existing yet
// included. A configuration not known yet, or older than the file of the
// configuration, as when a svc changes its install lines, says yes: the
// install command run reads the lines themselves, and changes nothing where
// they name another datastore. A template of a volume reading a key of p at
// its last render says yes too.
func mayInstallKeysOf(p naming.Path, st *instance.Status, cfg *instance.Config, configMtime func() time.Time, templateReadsFrom func(rid string) bool) bool {
	rids, up := upVolumeRIDs(st)
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
