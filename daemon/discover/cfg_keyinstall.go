package discover

import (
	"context"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/command"
)

type (
	// keyInstaller refreshes the keys a datastore installs in the volumes
	// of the local instances, when its configuration is fetched from a peer.
	//
	// The node writing a key refreshes its own volumes, in the process
	// writing it. The other nodes receive the new configuration through the
	// daemon, and nothing else would install the new content, or send the
	// signals the install lines declare: a haproxy reloading on a change of
	// its configuration key would keep the old one on every node but the
	// one the key was changed on.
	keyInstaller struct {
		mu      sync.Mutex
		running map[naming.Path]bool
		again   map[naming.Path]bool
	}
)

// keyInstallTimeout bounds an install, which signals resources, as a
// container, with timeouts of their own.
const keyInstallTimeout = 2 * time.Minute

var keyInstallCmdPath = func() string {
	p, err := os.Executable()
	if err != nil {
		return "/bin/false"
	}
	return p
}()

func newKeyInstaller() *keyInstaller {
	return &keyInstaller{running: make(map[naming.Path]bool), again: make(map[naming.Path]bool)}
}

// mayReceiveKeys says whether a local instance may install keys of a datastore
// shared with namespaces: an instance of a svc in one of them, with a volume
// or fs resource up.
//
// It reads what the daemon holds, and errs towards yes: whether a volume of
// the instance installs from this datastore is said by its install lines,
// which name the datastore relatively and with references, as "./cfg/{name}",
// and only the object they are evaluated in tells. The install command run
// when this says yes reads them, and changes nothing where they name another
// datastore.
func mayReceiveKeys(shares []string, local map[naming.Path]*instance.Status) bool {
	for p, st := range local {
		if st == nil || p.Kind != naming.KindSvc {
			continue
		}
		if !slices.Contains(shares, "*") && !slices.Contains(shares, p.Namespace) {
			continue
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
			if rst.Status.Is(status.Up, status.StandbyUp) {
				return true
			}
		}
	}
	return false
}

// onDataStoreConfigFetched refreshes the keys of the datastore p installed in
// the local volumes, when a local instance may receive them. A refresh asked
// while one runs for the same datastore is run once it ends, with the newest
// configuration.
func (t *Manager) onDataStoreConfigFetched(p naming.Path) {
	if !slices.Contains(naming.KindDataStore, p.Kind) {
		return
	}
	ki := t.keyInstaller
	ki.mu.Lock()
	if ki.running[p] {
		ki.again[p] = true
		ki.mu.Unlock()
		return
	}
	ki.running[p] = true
	ki.mu.Unlock()

	go func() {
		for {
			t.refreshInstalledKeys(p)
			ki.mu.Lock()
			if !ki.again[p] {
				delete(ki.running, p)
				ki.mu.Unlock()
				return
			}
			delete(ki.again, p)
			ki.mu.Unlock()
		}
	}()
}

func (t *Manager) refreshInstalledKeys(p naming.Path) {
	log := t.objectLogger(p)
	store, err := object.NewDataStore(p, object.WithVolatile(true))
	if err != nil {
		log.Warnf("cfg: refresh the installed keys of %s: %s", p, err)
		return
	}
	if !mayReceiveKeys(store.Shares(), instance.StatusData.GetByNode(t.localhost)) {
		log.Tracef("cfg: no local instance may install keys of %s: no refresh", p)
		return
	}
	ctx, cancel := context.WithTimeout(t.ctx, keyInstallTimeout)
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
