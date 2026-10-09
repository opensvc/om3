package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterlock"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

// ipAddrInfoKey is the resource status info an ip resource publishes its
// address under.
const ipAddrInfoKey = "ipaddr"

// NewAllocator returns the allocator of a network on a node, or nil when the
// network is one om allocates no address in.
func NewAllocator(nw Networker, nodename string) (*ipam.T, error) {
	i, ok := nw.(IPAMer)
	if !ok {
		return nil, nil
	}
	rng, err := i.AllocatableRange(nodename)
	if err != nil {
		return nil, fmt.Errorf("network %s: %w", nw.Name(), err)
	}
	if rng == nil {
		return nil, nil
	}
	if i, ok := nw.(ClusterWider); ok && i.IsClusterWide() {
		// No bridge answers for an address of the range, and no plugin
		// ever allocated in it.
		return &ipam.T{
			Name:        nw.Name(),
			Range:       rng,
			Dir:         ipam.StoreDir(nw.Name()),
			ClusterWide: true,
		}, nil
	}
	return &ipam.T{
		Name:    nw.Name(),
		Range:   rng,
		Gateway: ipam.Gateway(rng),
		Dir:     ipam.StoreDir(nw.Name()),
		// The record of the host-local plugin is read while it still hands
		// out addresses of this network, and never written: an address it
		// gave has no reservation of om's until a setup adopts it.
		PeerDirs: []string{filepath.Join(cniCacheDir, nw.Name())},
	}, nil
}

// Lookup returns the network of a name, or nil when no network has it.
func Lookup(name string) (Networker, []string, error) {
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0)
	for _, nw := range Networks(node) {
		if nw.Name() == name {
			return nw, nil, nil
		}
		names = append(names, nw.Name())
	}
	return nil, names, nil
}

// cniCacheDir is where the host-local plugin records the addresses it hands
// out.
const cniCacheDir = "/var/lib/cni/networks"

// setupIPAM records the addresses the resources of this node already hold, so
// the allocator hands out the addresses that are free rather than the ones it
// has no reservation for yet.
//
// It runs here rather than when a resource starts, because it has to be done
// once for a whole network and before the first allocation in it. A setup is
// where the once-per-network work of a node already happens, and it runs on
// daemon start and on every cluster configuration change.
//
// This matters most the first time a node allocates in a network whose
// addresses another allocator was handing out. The host-local plugin stops
// running then, so it stops releasing what it gave: every address it holds
// would be blocked for as long as its record lasts, and every address it holds
// would be free to hand out twice if that record were ignored. Neither, once
// the addresses in use are reservations of om's.
func setupIPAM(nws []Networker) error {
	reservations, err := localReservations()
	if err != nil {
		return err
	}
	installed, err := installedPaths()
	if err != nil {
		return err
	}
	nodename := hostname.Hostname()
	for _, nw := range nws {
		a, err := NewAllocator(nw, nodename)
		if err != nil {
			nw.Log().Warnf("ipam: %s", err)
			continue
		}
		if a == nil {
			continue
		}
		adopted, err := a.Adopt(reservations)
		if err != nil {
			return err
		}
		if adopted > 0 {
			nw.Log().Infof("ipam: adopted %d address(es) already held in this network", adopted)
		}
		reaped, err := a.Reap(func(key string) bool {
			p, ok := ipam.PathOfKey(key)
			if !ok {
				// A key of a shape this om does not write is not one it may
				// decide is gone.
				return true
			}
			return installed[p.String()]
		})
		if err != nil {
			return err
		}
		if reaped > 0 {
			nw.Log().Infof("ipam: released %d address(es) held for an object that no longer exists", reaped)
		}
		drained, left, err := a.DrainPeers()
		if err != nil {
			return err
		}
		if drained > 0 {
			nw.Log().Infof("ipam: dropped %d address(es) from the record of the plugin that used to allocate them", drained)
		}
		if left > 0 {
			nw.Log().Warnf("ipam: the record of the plugin that used to allocate in this network still holds %d address(es) om accounts for in no way, and they stay excluded. Remove them from %s once nothing uses them", left, filepath.Join(cniCacheDir, nw.Name()))
		}
	}
	return nil
}

// localReservations returns the addresses the ip resources of this node hold,
// read from the status every object caches locally.
//
// The cache of the host-local plugin cannot serve: it names the holder of an
// address by the pid of a network namespace, which says nothing about which
// resource that is, so an address adopted from it could never be released by
// the resource that stops. The object status names the resource, and it is on
// this node, which is the only node whose addresses matter here.
func localReservations() ([]ipam.Reservation, error) {
	paths, err := naming.InstalledPaths()
	if err != nil {
		return nil, err
	}
	l := make([]ipam.Reservation, 0)
	for _, p := range paths {
		status, err := loadInstanceStatus(p)
		if err != nil {
			// An object with no status yet holds no address yet.
			continue
		}
		for rid, rstat := range status.Resources {
			s, ok := rstat.Info[ipAddrInfoKey].(string)
			if !ok || s == "" {
				continue
			}
			ip := net.ParseIP(s)
			if ip == nil {
				continue
			}
			l = append(l, ipam.Reservation{IP: ip, Key: ipam.Key(p, rid)})
		}
	}
	return l, nil
}

// loadInstanceStatus reads the status an object cached, without evaluating it.
func loadInstanceStatus(p naming.Path) (instance.Status, error) {
	var data instance.Status
	b, err := os.ReadFile(filepath.Join(p.VarDir(), "status.json"))
	if err != nil {
		return data, err
	}
	err = json.Unmarshal(b, &data)
	return data, err
}

// installedPaths returns the objects configured on this node, by path.
func installedPaths() (map[string]bool, error) {
	paths, err := naming.InstalledPaths()
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(paths))
	for _, p := range paths {
		m[p.String()] = true
	}
	return m, nil
}

// AllocateFor reserves an address for a resource, refusing when the namespace
// has already taken all it claimed of the network.
//
// The claim is checked only when the reservation is a new one. An allocator is
// idempotent for a key, and it is asked again on every start, so a resource
// that already holds its address is not taking one more: refusing it there
// would stop an object the cluster let take the address in the first place,
// and a namespace that reached its limit could no longer restart what it runs.
func AllocateFor(ctx context.Context, i *ipam.T, p naming.Path, rid string) (net.IP, error) {
	return allocateFor(ctx, i, p, rid, nil)
}

// RedrawFor releases the address a resource holds, previous, and reserves
// another one.
//
// It is what a resource whose recorded address was taken away asks for: the
// address it holds is the one it is to give up, so neither the reservation of
// this node nor the one it holds on another node is taken back.
//
// A redraw that fails holds previous again, so the next one still knows what
// to give up.
func RedrawFor(ctx context.Context, i *ipam.T, p naming.Path, rid string, previous net.IP) (net.IP, error) {
	key := ipam.Key(p, rid)
	if err := i.Free(key); err != nil {
		return nil, err
	}
	ip, err := allocateFor(ctx, i, p, rid, previous)
	if err != nil {
		if _, adoptErr := i.Adopt([]ipam.Reservation{{IP: previous, Key: key}}); adoptErr != nil {
			return nil, errors.Join(err, adoptErr)
		}
		return nil, err
	}
	return ip, nil
}

func allocateFor(ctx context.Context, i *ipam.T, p naming.Path, rid string, previous net.IP) (net.IP, error) {
	key := ipam.Key(p, rid)
	held, err := i.Allocated(key)
	if err != nil {
		return nil, err
	}
	if held != nil {
		return held, nil
	}
	exclude := make([]net.IP, 0)
	if previous != nil {
		exclude = append(exclude, previous)
	}
	if i.ClusterWide {
		// Every node draws from the range, and what the others drew is read
		// before drawing: two nodes doing it at once would both read a
		// cluster without the address the other is about to take.
		release, err := lockNetwork(ctx, i.Name, key)
		if err != nil {
			return nil, fmt.Errorf("network %s: %w", i.Name, err)
		}
		defer release()
		ip, others, err := adoptClusterAddr(ctx, i, key, previous == nil)
		if err != nil {
			return nil, err
		}
		if ip != nil {
			return ip, nil
		}
		exclude = append(exclude, others...)
	}
	if ok, why, err := ClaimFits(ctx, i.Name, p.Namespace, p, rid); err != nil {
		return nil, fmt.Errorf("network %s claim check: %w", i.Name, err)
	} else if !ok {
		return nil, fmt.Errorf("network %s: %s", i.Name, why)
	}
	if len(exclude) > 0 {
		inUse := i.InUse
		i.InUse = func() ([]net.IP, error) {
			if inUse == nil {
				return exclude, nil
			}
			l, err := inUse()
			return append(l, exclude...), err
		}
	}
	return i.Allocate(key)
}

// lockNetworkWait is how long an allocation waits for another node drawing
// from the same network.
const lockNetworkWait = time.Minute

// lockNetwork takes the cluster lock of a network, and returns what releases
// it.
var lockNetwork = func(ctx context.Context, networkName, key string) (func(), error) {
	lock, err := clusterlock.Acquire(ctx, "network/"+networkName, clusterlock.Options{Holder: key, Wait: lockNetworkWait})
	if err != nil {
		return nil, err
	}
	return func() {
		// A release that fails leaves the lock to its lease, which only
		// delays the next allocation.
		_ = lock.Release(context.Background())
	}, nil
}

// clusterAddrs reads the addresses the resources of the cluster hold in a
// network, by reservation key.
//
// Two readings make it. The status of the objects, which the daemon
// replicates, has every address a resource reports, on the nodes alive or
// not. The reservation store of every node alive has the addresses drawn and
// not reported yet, as the one the node that held the network lock before
// this one just drew: the object it drew it for publishes its status once its
// action is over.
var clusterAddrs = func(ctx context.Context, networkName string) (map[string][]net.IP, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	m, err := statusAddrs(ctx, c, networkName)
	if err != nil {
		return nil, err
	}
	reserved, err := storeAddrs(ctx, c, networkName)
	if err != nil {
		return nil, err
	}
	for key, ips := range reserved {
		for _, ip := range ips {
			if !slices.ContainsFunc(m[key], ip.Equal) {
				m[key] = append(m[key], ip)
			}
		}
	}
	return m, nil
}

// statusAddrs reads the addresses the resources of the cluster report in a
// network, by reservation key, from the status the daemon replicates.
func statusAddrs(ctx context.Context, c *client.T, networkName string) (map[string][]net.IP, error) {
	resp, err := c.GetNetworkIPWithResponse(ctx, &api.GetNetworkIPParams{Name: &networkName})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("read the %s network addresses: unexpected status code %d", networkName, resp.StatusCode())
	}
	m := make(map[string][]net.IP)
	for _, item := range resp.JSON200.Items {
		p, err := naming.ParsePath(item.Path)
		if err != nil {
			continue
		}
		ip := net.ParseIP(item.IP)
		if ip == nil {
			continue
		}
		key := ipam.Key(p, item.RID)
		m[key] = append(m[key], ip)
	}
	return m, nil
}

// storeAddrs reads the addresses every node alive reserved in a network, by
// reservation key, from their reservation stores.
//
// A node the daemon has no data of is not alive, and is skipped. One alive
// that does not answer fails the reading: what it drew is what the reading
// is for.
func storeAddrs(ctx context.Context, c *client.T, networkName string) (map[string][]net.IP, error) {
	ccfg, err := object.NewCluster(object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	nodenames, err := ccfg.Nodes()
	if err != nil {
		return nil, err
	}
	m := make(map[string][]net.IP)
	for _, nodename := range nodenames {
		resp, err := c.GetNodeNetworkReservationsWithResponse(ctx, nodename, &api.GetNodeNetworkReservationsParams{Name: &networkName})
		if err != nil {
			return nil, fmt.Errorf("read the %s network reservations of %s: %w", networkName, nodename, err)
		}
		switch {
		case resp.JSON200 != nil:
		case resp.StatusCode() == http.StatusNotFound:
			continue
		default:
			return nil, fmt.Errorf("read the %s network reservations of %s: unexpected status code %d", networkName, nodename, resp.StatusCode())
		}
		for _, item := range resp.JSON200.Items {
			p, err := naming.ParsePath(item.Path)
			if err != nil {
				continue
			}
			ip := net.ParseIP(item.IP)
			if ip == nil {
				continue
			}
			key := ipam.Key(p, item.RID)
			m[key] = append(m[key], ip)
		}
	}
	return m, nil
}

// adoptClusterAddr reserves on this node the address the resource holds on
// another node of a cluster-wide network, and returns it, nil when it holds
// none or adopt is false. Otherwise it returns the addresses the other
// resources hold anywhere, which the allocation keeps clear of: this node has
// no reservation of theirs.
//
// So a failover object moving to another node takes its address along, which
// is what a floating address is for, rather than drawing one the clients of
// the address it had do not know.
//
// The cluster is read through the daemon, without which the addresses of the
// other nodes are unknown: an allocation then fails rather than hand out an
// address another node may hold.
func adoptClusterAddr(ctx context.Context, i *ipam.T, key string, adopt bool) (net.IP, []net.IP, error) {
	held, err := clusterAddrs(ctx, i.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("network %s: read the addresses the cluster holds: %w", i.Name, err)
	}
	var own net.IP
	others := make([]net.IP, 0)
	for k, ips := range held {
		if k != key || !adopt {
			others = append(others, ips...)
			continue
		}
		for _, ip := range ips {
			switch {
			case own == nil:
				own = ip
			case !own.Equal(ip):
				return nil, nil, fmt.Errorf("network %s: %s holds both %s and %s on the nodes of the cluster: release one", i.Name, key, own, ip)
			}
		}
	}
	if own == nil {
		return nil, others, nil
	}
	if _, err := i.Adopt([]ipam.Reservation{{IP: own, Key: key}}); err != nil {
		return nil, nil, err
	}
	ip, err := i.Allocated(key)
	if err != nil {
		return nil, nil, err
	}
	if ip == nil || !ip.Equal(own) {
		return nil, nil, fmt.Errorf("network %s: %s is reserved on this node for another resource than %s, which holds it on another node", i.Name, own, key)
	}
	return ip, nil, nil
}

// ReleaseRemoved releases the addresses this node holds for the resources of
// an object that its configuration no longer has, the resources of rids being
// the ones it still has.
//
// A resource is released when it is unprovisioned, which a section taken
// away from the configuration skips: its reservation would stay, out of the
// pool and counted against the claim of the namespace, until the object is
// deleted.
func ReleaseRemoved(p naming.Path, rids map[string]bool) (int, error) {
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return 0, err
	}
	nodename := hostname.Hostname()
	released := 0
	var errs error
	for _, nw := range Networks(node) {
		a, err := NewAllocator(nw, nodename)
		if err != nil || a == nil {
			continue
		}
		n, err := a.Reap(func(key string) bool {
			kp, ok := ipam.PathOfKey(key)
			if !ok || kp.String() != p.String() {
				return true
			}
			_, rid, _ := strings.Cut(key, "!")
			return rids[rid]
		})
		released += n
		errs = errors.Join(errs, err)
	}
	return released, errs
}
