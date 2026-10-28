package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/file"
)

const (
	// CNIVersion is the version we write in the generated CNI network configuration files
	CNIVersion = "0.3.0"
)

var (
	ErrInvalidType = errors.New("invalid network type")
)

// setupFirewall applies the firewall rules of the networks, replaced by the
// tests.
var setupFirewall = setupFW

// Setup sets up the networks named, all of them when none is named, and the
// firewall rules of every network.
//
// The firewall rules of the networks are one ruleset, rewritten whole: rendered
// from the networks named only, as the daemon sets up the one whose
// configuration changed, it dropped the rules of the others, and the
// containers of their networks lost their way out.
func Setup(n *object.Node, names ...string) error {
	errs := make([]error, 0)
	dir, err := mkCNIConfigDir(n)
	if err != nil {
		return err
	}
	cluster, err := object.NewCluster()
	if err != nil {
		return err
	}
	all := Networks(n)
	nws := named(all, names)
	invalid := Check(all)
	needCommit := make([]string, 0)
	kops := make(keyop.L, 0)
	for _, nw := range nws {
		if err := invalid[nw.Name()]; err != nil {
			nw.Log().Errorf("network setup: %s", err)
			errs = append(errs, err)
			continue
		}
		if err := setupNetwork(n, nw, dir); err != nil {
			nw.Log().Errorf("network setup: %s", err)
			errs = append(errs, err)
		}
		if nw.NeedCommit() {
			needCommit = append(needCommit, nw.Name())
			kops = append(kops, nw.Kops()...)
		}
	}
	if len(needCommit) > 0 {
		n.Log().Infof("network setup: commit config changes on %s", strings.Join(needCommit, ","))
		cluster.Config().Set(kops...)
	}
	if err := setupIPAM(nws); err != nil {
		errs = append(errs, err)
	}
	if err := setupFirewall(n, all); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("network setup: %s", errs)
	}
	return nil
}

// named returns the networks named, all of them when none is named.
func named(nws []Networker, names []string) []Networker {
	if len(names) == 0 {
		return nws
	}
	l := make([]Networker, 0, len(names))
	for _, nw := range nws {
		if slices.Contains(names, nw.Name()) {
			l = append(l, nw)
		}
	}
	return l
}

func intersect(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

// Check returns, by name, the error of each network of the list that cannot
// be used: its ranges do not parse or leave its segment, or they overlap the
// addresses of a network before it in the list, which is the order of the
// configuration. The network declared first keeps working, and the one
// declared after it is the one refused, so a network added overlapping
// another does not take the other down with it.
//
// A network whose segment does not parse is left to IsValid, which the setup
// skips it on.
func Check(nws []Networker) map[string]error {
	errs := make(map[string]error)
	var usables []usableNetwork
	for _, nw := range nws {
		_, ipnet, err := net.ParseCIDR(nw.Network())
		if err != nil {
			continue
		}
		if prefix, err := netip.ParsePrefix(nw.Network()); err != nil {
			errs[nw.Name()] = err
			continue
		} else if prefix.Addr().String() != ipnet.IP.String() {
			// ex: 172.10.10.0/22 prefix addr 172.10.10.0 does not match the prefix length (expected 172.10.8.0)
			errs[nw.Name()] = fmt.Errorf("%s prefix addr %s does not match the prefix length (expected %s)", prefix, prefix.Addr(), ipnet.IP)
			continue
		}
		pools := []ipam.Pool{ipam.PoolOf(ipnet)}
		if i, ok := nw.(Pooler); ok {
			if pools, err = i.Pools(); err != nil {
				errs[nw.Name()] = err
				continue
			}
		}
		this := usableNetwork{nw: nw, ipnet: ipnet, pools: pools}
		if err := this.overlap(usables); err != nil {
			errs[nw.Name()] = err
			continue
		}
		usables = append(usables, this)
	}
	return errs
}

// usableNetwork is a network Check found usable, with the addresses it hands
// out: the ranges of a network handing out some of its segment only, as two
// lan networks of the same segment do, else the whole of it.
type usableNetwork struct {
	nw    Networker
	ipnet *net.IPNet
	pools []ipam.Pool
}

// overlap says which addresses of the networks found usable before the
// addresses of t overlap, nil when none do.
func (t usableNetwork) overlap(usables []usableNetwork) error {
	for _, other := range usables {
		if !intersect(t.ipnet, other.ipnet) {
			continue
		}
		for _, p := range t.pools {
			for _, o := range other.pools {
				if p.Overlaps(o) {
					return fmt.Errorf("%s overlaps %s (%s)", p, o, other.nw.Name())
				}
			}
		}
	}
	return nil
}

func setupNetwork(n *object.Node, nw Networker, dir string) error {
	if IsDisabled(nw) {
		nw.Log().Tracef("network setup: skip disabled network")
		return nil
	}
	if !IsValid(nw) {
		nw.Log().Infof("network setup: skip invalid network")
		return nil
	}
	if i, ok := nw.(Setuper); ok {
		if err := i.Setup(); err != nil {
			return err
		}
	}
	if err := setupNetworkCNI(n, dir, nw); err != nil {
		return err
	}
	return nil
}

func mkCNIConfigDir(n *object.Node) (string, error) {
	dir, err := n.CNIConfig()
	if err != nil {
		return dir, err
	}
	if file.Exists(dir) {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0600); err != nil {
		return dir, err
	}
	return dir, nil
}

func CNIConfigFile(dir string, nw Networker) string {
	return filepath.Join(dir, nw.Name()+".conf")
}

func setupNetworkCNI(n *object.Node, dir string, nw Networker) error {
	i, ok := nw.(CNIer)
	if !ok {
		return nil
	}
	data, err := i.CNIConfigData()
	if err != nil {
		return err
	}
	p := CNIConfigFile(dir, nw)
	nw.Log().Infof("cni %s write", p)
	return writeCNIConfig(p, data)
}

func writeCNIConfig(fpath string, data interface{}) error {
	tmp, err := os.CreateTemp(filepath.Dir(fpath), "."+filepath.Base(fpath)+".")
	if err != nil {
		return err
	}
	if b, err := json.MarshalIndent(data, "", "   "); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	} else if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	} else {
		_ = tmp.Close()
		return os.Rename(tmp.Name(), fpath)
	}
}
