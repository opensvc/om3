package network

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"net"
	"strings"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/clusterip"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/stringslice"
)

type (
	T struct {
		driver     string
		name       string
		isImplicit bool
		needCommit bool
		log        *zerolog.Logger
		noder      Noder
	}

	Noder interface {
		MergedConfig() *xconfig.T
		Log() *zerolog.Logger
		Nodes() []string
	}
	Networker interface {
		// SetDriver sets the driver name, which is obtained from the
		// "type" keyword in the network configuration.
		SetDriver(string)

		// SetName sets the network name. See Name().
		SetName(string)

		// SetImplicit sets isImplicit. See IsImplicit().
		SetImplicit(bool)

		// SetNeedCommit can be called by drivers to signal network
		// configuration changes are staged and // need to be written
		// to file.
		SetNeedCommit(bool)

		// SetNoder stores the Noder
		SetNoder(Noder)

		// IsImplicit returns true if the network is a builtin with
		// no override section in the configuration.
		IsImplicit() bool

		// IsValid returns true if the network is a valid CIDR string.
		IsValid() bool

		// IsIP6 returns true if the network is a CIDR representation
		// of a IPv6 network.
		IsIP6() bool

		// Name returns the name of the network. Which is the part
		// after the dash in the configuration section name.
		Name() string

		// Network returns the CIDR representation of the network.
		Network() string

		// NeedCommit return true if the network configuration cache
		// has staged changes. This can be used by Networks() users to
		// do one commit per action instead of one per network.
		NeedCommit() bool

		// Type return the driver name.
		Type() string

		// Usage returns the usage metrics of the network.
		Usage() (StatusUsage, error)

		FilterIPs(clusterip.L) clusterip.L

		// AllowEmptyNetwork can be defined by a specific driver to
		// announce the network core that empty network option is fine.
		// For one, the loopback driver uses that.
		AllowEmptyNetwork() bool

		// Config is a wrapper for the noder MergedConfig
		Config() *xconfig.T

		// Log returns a zerolog Logger configured to add the network
		// name to log entries.
		Log() *zerolog.Logger

		// Nodes is a wrapper for the noder Nodes, which returns the
		// list of cluster nodes to make the network available on.
		Nodes() []string
	}
	Setuper interface {
		Setup() error
	}
	CNIer interface {
		CNIConfigData() (interface{}, error)
	}
)

var (
	drivers = make(map[string]func() Networker)
)

func (t *T) Log() *zerolog.Logger {
	if t.log == nil {
		log := t.noder.Log().With().Str("name", t.name).Logger()
		t.log = &log
	}
	return t.log
}

func (t T) Nodes() []string {
	return t.noder.Nodes()
}

func NewTyped(name string, networkType string, noder Noder) Networker {
	fn, ok := drivers[networkType]
	if !ok {
		return nil
	}
	t := fn()
	t.SetName(name)
	t.SetDriver(networkType)
	t.SetNoder(noder)
	return t.(Networker)
}

func New(name string, noder Noder) Networker {
	networkType := cString(noder.MergedConfig(), name, "type")
	return NewTyped(name, networkType, noder)
}

func Register(t string, fn func() Networker) {
	drivers[t] = fn
}

func (t T) Name() string {
	return t.name
}

func (t *T) SetName(name string) {
	t.name = name
}

func (t T) NeedCommit() bool {
	return t.needCommit
}

func (t *T) SetNeedCommit(v bool) {
	t.needCommit = v
}

func (t *T) SetDriver(driver string) {
	t.driver = driver
}

func (t T) Type() string {
	return t.driver
}

func (t *T) Config() *xconfig.T {
	return t.noder.MergedConfig()
}

func (t *T) SetNoder(noder Noder) {
	t.noder = noder
}

func (t T) FilterIPs(ips clusterip.L) clusterip.L {
	l := make(clusterip.L, 0)
	if ipnet, err := t.IPNet(); err != nil {
		return l
	} else {
		return ips.ByNetwork(ipnet)
	}
}

func (t T) key(option string) key.T {
	return key.New("network#"+t.name, option)
}

func (t *T) GetString(s string) string {
	k := t.key(s)
	return t.Config().GetString(k)
}

func (t *T) Set(option, value string) error {
	k := t.key(option)
	kop := keyop.T{
		Key:   k,
		Op:    keyop.Set,
		Value: value,
	}
	if err := t.Config().Set(kop); err != nil {
		return err
	}
	t.needCommit = true
	return nil
}

func (t *T) GetSlice(s string) []string {
	k := t.key(s)
	return t.Config().GetSlice(k)
}

func (t *T) Tables() []string {
	return t.GetSlice("tables")
}

func (t T) AllowEmptyNetwork() bool {
	return false
}

// IsValidNetwork returns true if the network configuration is sane enough to setup.
func (t T) IsValid() bool {
	s := t.Network()
	if s == "" && t.AllowEmptyNetwork() {
		return true
	}
	if _, _, err := net.ParseCIDR(s); err != nil {
		return false
	}
	return true
}

func (t T) IsIP6() bool {
	ip, _, err := net.ParseCIDR(t.Network())
	if err != nil {
		return false
	}
	return ip.To4() == nil
}

func (t *T) Network() string {
	return t.GetString("network")
}

func (t *T) IPsPerNode() (int, error) {
	i, err := t.Config().Eval(cKey(t.Name(), "ips_per_node"))
	if err != nil {
		return 0, err
	}
	return i.(int), nil
}

func (t *T) SetImplicit(v bool) {
	t.isImplicit = v
}

func (t *T) IsImplicit() bool {
	return t.isImplicit
}

func namesInConfig(noder Noder) []string {
	l := make([]string, 0)
	for _, s := range noder.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "network#") {
			continue
		}
		l = append(l, s[8:])
	}
	return l
}

func (t T) IPNet() (*net.IPNet, error) {
	_, ipnet, err := net.ParseCIDR(t.Network())
	return ipnet, err
}

//
// NodeSubnet returns the network subnet assigned to a cluster node, as a *net.IPNet.
// This subnet is usually found in the network configuration, as a subnet@<nodename>
// option. If not found there, allocate and write one.
//
// The subnet allocator uses ips_per_node to compute a netmask (narrower than the
// network mask).
// The subnet first ip is computed using the position of the node in the cluster
// nodes list.
//
// Example:
// With
//    cluster.nodes = n1 n2 n3
//    net1.network = 10.0.0.0/24
//    net1.ips_per_node = 64
// =>
//    subnet@n1 = 10.0.0.0/26
//    subnet@n2 = 10.0.0.64/26
//    subnet@n3 = 10.0.0.128/26
//
func (t *T) NodeSubnet(nodename string) (*net.IPNet, error) {
	if nodename == "" {
		return nil, fmt.Errorf("empty nodename")
	}
	if subnet := t.GetSubnetAs(nodename); subnet == "" {
		// no configured subnet yet => allocate one
	} else if _, ipnet, err := net.ParseCIDR(subnet); err != nil {
		return nil, err
	} else if ipnet != nil {
		t.Log().Debug().Msgf("subnet %s previously assigned to node %s", ipnet, nodename)
		return ipnet, nil
	}

	idx := stringslice.Index(nodename, t.Nodes())
	ipsPerNode, err := t.IPsPerNode()
	ipsPerNode = 1 << bits.Len(uint(ipsPerNode)-1)
	if err != nil {
		return nil, err
	}
	ipnet, err := t.IPNet()
	if err != nil {
		return nil, err
	}
	if ipnet == nil {
		return nil, fmt.Errorf("node %s subnet: empty network", nodename)
	}
	ip := ipnet.IP
	IncIPN(ip, ipsPerNode*idx)
	_, ipnetBits := ipnet.Mask.Size()
	subnetOnes := ipnetBits - bits.Len(uint(ipsPerNode)-1)
	mask := net.CIDRMask(subnetOnes, ipnetBits)
	subnetIPNet := &net.IPNet{
		IP:   ip,
		Mask: mask,
	}
	if err := t.Set("subnet@"+nodename, subnetIPNet.String()); err != nil {
		t.Log().Warn().Err(err).Msgf("assign subnet %s to node %s", subnetIPNet, nodename)
	} else {
		t.Log().Info().Msgf("assign subnet %s to node %s", subnetIPNet, nodename)
	}
	return subnetIPNet, nil
}

func (t *T) GetSubnetAs(nodename string) string {
	k := t.key("subnet")
	i, err := t.Config().EvalAs(k, nodename)
	if err != nil {
		return ""
	}
	if subnet, ok := i.(string); ok {
		return subnet
	}
	return ""
}

func getClusterIPList(c *client.T, selector string) (clusterip.L, error) {
	var (
		err           error
		b             []byte
		clusterStatus cluster.Status
	)
	b, err = c.NewGetDaemonStatus().
		SetSelector(selector).
		Do()
	if err != nil {
		return clusterip.L{}, err
	}
	err = json.Unmarshal(b, &clusterStatus)
	if err != nil {
		return clusterip.L{}, err
	}
	return clusterip.NewL().Load(clusterStatus), nil
}

func Networks(noder Noder) []Networker {
	l := make([]Networker, 0)
	hasLO := false
	hasDefault := false

	for _, name := range namesInConfig(noder) {
		p := New(name, noder)
		if p == nil {
			continue
		}
		if p.Type() == "shm" {
			hasLO = true
		}
		if p.Name() == "default" {
			hasDefault = true
		}
		l = append(l, p)
	}
	if !hasLO {
		p := NewTyped("lo", "lo", noder)
		p.SetImplicit(true)
		l = append(l, p)
	}
	if !hasDefault {
		p := NewTyped("default", "bridge", noder)
		p.SetImplicit(true)
		l = append(l, p)
	}
	return l
}
