// Package ipam allocates the addresses of an om network on one node.
//
// The allocation is node local, which is what makes it safe without a lock
// held across the cluster. A routed_bridge network gives every node a subnet
// of its own, so two nodes never draw from the same addresses. A bridge
// network gives every node the whole subnet, but its addresses are node local
// and not routable, so the same address on two nodes never meets.
package ipam

import (
	"fmt"
	"hash/fnv"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

// StoreDir returns where the reservations of a network are recorded.
//
// One place, so the allocator a resource builds and the adoption a network
// setup runs record in the same directory.
func StoreDir(name string) string {
	return filepath.Join(rawconfig.Paths.Var, "ipam", name)
}

type (
	// T allocates in one network, on one node.
	T struct {
		// Name is the network the addresses are drawn from.
		Name string

		// Range is the addresses this node draws from.
		Range *net.IPNet

		// Gateway is not allocated: it is the address of the bridge.
		Gateway net.IP

		// Dir is where the reservations are recorded, one file per address.
		Dir string

		// PeerDirs are the reservation stores of the allocators sharing this
		// range, read and never written. While ip.cni is still served by the
		// host-local plugin, its store is one of these: an address it handed
		// out has no reservation here, and would be handed out twice.
		PeerDirs []string

		// InUse reports the addresses the cluster says are taken. The daemon
		// replicates the resource status of every instance, so this sees the
		// addresses of objects whose reservation file this node cannot read.
		//
		// Neither network driver needs it: a routed_bridge gives this node a
		// range no other node draws from, and the addresses of a bridge are
		// node local and not routable. It is here for a network type that is
		// neither.
		InUse func() ([]net.IP, error)
	}

	// Reservation is an address already held by a resource, which an adoption
	// records so the resource keeps it.
	Reservation struct {
		IP  net.IP
		Key string
	}
)

// maxProbes bounds the walk over the candidates of a range.
//
// A range is walked from a candidate the key decides, and a full walk of an
// ipv6 range would not end. A network with more free addresses than this and
// none in the first probes is a network with a leak, not a network that is
// full.
const maxProbes = 4096

// Allocate returns the address reserved for key, reserving it when it has
// none.
//
// The same key draws the same address as long as it stays free, so an object
// keeps its address across restarts and its name keeps meaning what it meant.
// The reservation is a file created exclusively, which is the whole of the
// locking: one node draws from this range, so there is no other writer.
func (t *T) Allocate(key string) (net.IP, error) {
	if t.Range == nil {
		return nil, fmt.Errorf("network %s allocates no address on this node", t.Name)
	}
	if ip, err := t.Allocated(key); err != nil {
		return nil, err
	} else if ip != nil {
		return ip, nil
	}
	taken, err := t.taken()
	if err != nil {
		return nil, err
	}
	ones, bits := t.Range.Mask.Size()
	size := new(big.Int).Lsh(big.NewInt(1), uint(bits-ones))
	first := ipToInt(t.Range.IP)
	offset := new(big.Int).Mod(keyOffset(key), size)

	probes := maxProbes
	if size.IsInt64() && size.Int64() < int64(probes) {
		probes = int(size.Int64())
	}
	for i := 0; i < probes; i++ {
		ip := intToIP(new(big.Int).Add(first, offset), t.Range.IP.To4() != nil)
		if t.isAllocatable(ip, size) && !taken[ip.String()] {
			if ok, err := t.reserve(ip, key); err != nil {
				return nil, err
			} else if ok {
				return ip, nil
			}
		}
		offset.Add(offset, big.NewInt(1))
		offset.Mod(offset, size)
	}
	return nil, fmt.Errorf("network %s: no free address in %s after %d probes", t.Name, t.Range, probes)
}

// Adopt records the addresses resources already hold, so the allocator hands
// out the addresses that are free rather than the addresses that have no
// reservation yet.
//
// It is how the allocation of a network moves from an allocator that was
// keeping its own record. The record of the host-local plugin cannot serve:
// it names the holder of an address by the pid of a network namespace, which
// says nothing about which resource that is. The cluster status can, since it
// carries the object and the rid alongside the address.
//
// An address already reserved is left alone, whoever holds it, so an adoption
// run twice changes nothing and an adoption run late does not take an address
// from the resource that drew it. The count returned is of the addresses this
// call recorded, which is how a setup says whether it had anything to adopt.
func (t *T) Adopt(reservations []Reservation) (int, error) {
	n := 0
	for _, reservation := range reservations {
		if reservation.IP == nil || reservation.Key == "" {
			continue
		}
		if t.Range != nil && !t.Range.Contains(reservation.IP) {
			continue
		}
		ok, err := t.reserve(reservation.IP, reservation.Key)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// Allocated returns the address key holds, or nil when it holds none.
func (t *T) Allocated(key string) (net.IP, error) {
	entries, err := os.ReadDir(t.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		ip := net.ParseIP(entry.Name())
		if ip == nil {
			continue
		}
		if held, err := t.holder(entry.Name()); err != nil {
			return nil, err
		} else if held == key {
			return ip, nil
		}
	}
	return nil, nil
}

// Free releases the address key holds, and does nothing when it holds none.
//
// Only the holder frees an address: the file names who took it, and a
// reservation another key made is left alone.
func (t *T) Free(key string) error {
	ip, err := t.Allocated(key)
	if err != nil {
		return err
	}
	if ip == nil {
		return nil
	}
	err = os.Remove(filepath.Join(t.Dir, ip.String()))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// reserve creates the reservation of an address, and reports whether this
// call is the one that created it.
func (t *T) reserve(ip net.IP, key string) (bool, error) {
	if err := os.MkdirAll(t.Dir, 0755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(filepath.Join(t.Dir, ip.String()), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if os.IsExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.WriteString(key + "\n"); err != nil {
		return false, err
	}
	return true, nil
}

// holder returns the key an address is reserved for.
func (t *T) holder(addr string) (string, error) {
	b, err := os.ReadFile(filepath.Join(t.Dir, addr))
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// taken returns the addresses no allocation may draw: the ones reserved here,
// the ones reserved by an allocator sharing the range, and the ones the
// cluster reports in use.
func (t *T) taken() (map[string]bool, error) {
	m := make(map[string]bool)
	for _, dir := range append([]string{t.Dir}, t.PeerDirs...) {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if ip := net.ParseIP(entry.Name()); ip != nil {
				m[ip.String()] = true
			}
		}
	}
	if t.InUse != nil {
		ips, err := t.InUse()
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			m[ip.String()] = true
		}
	}
	return m, nil
}

// isAllocatable reports whether an address of the range may be handed to an
// object.
//
// The first address of a range names the range, the last of an ipv4 range is
// its broadcast address, and the gateway answers for the bridge.
func (t *T) isAllocatable(ip net.IP, size *big.Int) bool {
	if !t.Range.Contains(ip) {
		return false
	}
	offset := new(big.Int).Sub(ipToInt(ip), ipToInt(t.Range.IP))
	if offset.Sign() == 0 {
		return false
	}
	if ip.To4() != nil && offset.Cmp(new(big.Int).Sub(size, big.NewInt(1))) == 0 {
		return false
	}
	if t.Gateway != nil && t.Gateway.Equal(ip) {
		return false
	}
	return true
}

// keyOffset returns where in a range the walk for a key starts.
//
// It is a hash of the key rather than the first free address, so the same
// object draws the same address every time without a record of what it drew
// last: a restart does not renumber it, and its name keeps resolving to the
// same place.
func keyOffset(key string) *big.Int {
	h := fnv.New64a()
	h.Write([]byte(key))
	return new(big.Int).SetUint64(h.Sum64())
}

func ipToInt(ip net.IP) *big.Int {
	if v4 := ip.To4(); v4 != nil {
		return new(big.Int).SetBytes(v4)
	}
	return new(big.Int).SetBytes(ip.To16())
}

func intToIP(i *big.Int, isV4 bool) net.IP {
	size := 16
	if isV4 {
		size = 4
	}
	b := i.Bytes()
	if len(b) > size {
		b = b[len(b)-size:]
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return net.IP(out)
}

// Gateway returns the address the bridge of a range answers for, which is the
// first address of the range plus one.
//
// Both network drivers derive the address of their bridge that way, from the
// range they hand out: the bridge driver from the whole subnet, the
// routed_bridge driver from the subnet of the node. So the allocator has one
// rule to keep and no second place for it to drift from.
func Gateway(rng *net.IPNet) net.IP {
	if rng == nil {
		return nil
	}
	return intToIP(new(big.Int).Add(ipToInt(rng.IP), big.NewInt(1)), rng.IP.To4() != nil)
}

// Key returns the reservation key of an ip resource.
//
// An address belongs to a resource, not to an object: an instance holds as
// many ip resources as it needs, several of them in one network, and each has
// an address of its own. Keying on the object alone would have ip#0 and ip#1
// draw the same one.
func Key(p naming.Path, rid string) string {
	return p.String() + "!" + rid
}
