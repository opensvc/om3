package network

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"net"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/clusterip"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/stringslice"
)

type (
	T struct {
		driver            string
		name              string
		network           string
		isImplicit        bool
		needCommit        bool
		allowEmptyNetwork bool

		log   *plog.Logger
		noder Noder
	}

	Noder interface {
		MergedConfig() *xconfig.T
		Log() *plog.Logger
		Nodes() ([]string, error)
	}
	Networker interface {
		// SetDriver sets the driver name, which is obtained from the
		// "type" keyword in the network configuration.
		SetDriver(string)

		// SetName sets the network name. See Name().
		SetName(string)

		// SetImplicit sets isImplicit. See IsImplicit().
		SetImplicit(bool)

		// SetNetwork sets the network CIDR. Normally read from the
		// merged cluster configuration.
		SetNetwork(string)

		// SetNeedCommit can be called by drivers to signal network
		// configuration changes are staged and // need to be written
		// to file.
		SetNeedCommit(bool)

		// SetNoder stores the Noder
		SetNoder(Noder)

		// IsImplicit returns true if the network is a builtin with
		// no override section in the configuration.
		IsImplicit() bool

		// IsIP6 returns true if the network is a CIDR representation
		// of a IPv6 network.
		IsIP6() bool

		// Name returns the name of the network. Which is the part
		// after the dash in the configuration section name.
		Name() string

		// Network returns the CIDR representation of the network.
		Network() string

		// IPNet returns the result of ParseCIDR() on the Network()
		// CIDR string, without the net.IP
		IPNet() (*net.IPNet, error)

		// NeedCommit return true if the network configuration cache
		// has staged changes. This can be used by Networks() users to
		// do one commit per action instead of one per network.
		NeedCommit() bool

		// Type return the driver name.
		Type() string

		FilterIPs(clusterip.L) clusterip.L

		AllowEmptyNetwork() bool

		// Config is a wrapper for the noder MergedConfig
		Config() *xconfig.T

		// Log returns a zerolog Logger configured to add the network
		// name to log entries.
		Log() *plog.Logger

		// Nodes is a wrapper for the noder Nodes, which returns the
		// list of cluster nodes to make the network available on.
		Nodes() ([]string, error)

		NodeSubnet(nodename string) (*net.IPNet, error)
		NodeSubnetIP(nodename string) (net.IP, error)
	}
	Setuper interface {
		Setup() error
	}
	CNIer interface {
		CNIConfigData() (interface{}, error)
	}
	logger interface {
		Log() *plog.Logger
	}
)

func (t *T) Log() *plog.Logger {
	if t.log == nil {
		t.log = t.noder.Log().
			Attr("netName", t.name).
			Attr("netDriver", t.driver).
			Attr("netNetwork", t.network).
			Attr("netImplicit", t.isImplicit).
			WithPrefix("network: " + t.name + ": ")
	}
	return t.log
}

func (t T) Nodes() ([]string, error) {
	return t.noder.Nodes()
}

func NewTyped(name, networkType, networkNetwork string, noder Noder) Networker {
	fn := Driver(networkType)
	if fn == nil {
		return nil
	}
	t := fn()
	t.SetName(name)
	t.SetDriver(networkType)
	t.SetNetwork(networkNetwork)
	t.SetNoder(noder)
	return t.(Networker)
}

func NewFromNoder(name string, noder Noder) Networker {
	config := noder.MergedConfig()
	networkType := cString(config, name, "type")
	networkNetwork := cString(config, name, "network")
	return NewTyped(name, networkType, networkNetwork, noder)
}

func Driver(t string) func() Networker {
	drvID := driver.NewID(driver.GroupNetwork, t)
	i := driver.Get(drvID)
	if i == nil {
		return nil
	}
	if a, ok := i.(func() Networker); ok {
		return a
	}
	return nil
}

func (t T) Name() string {
	return t.name
}

func (t *T) SetName(name string) {
	t.name = name
}

func (t *T) SetDriver(name string) {
	t.driver = name
}

func (t T) NeedCommit() bool {
	return t.needCommit
}

func (t *T) SetNeedCommit(v bool) {
	t.needCommit = v
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

func (t *T) GetInt(s string) int {
	k := t.key(s)
	return t.Config().GetInt(k)
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
	if err := t.Config().PrepareSet(kop); err != nil {
		return err
	}
	t.needCommit = true
	return nil
}

func (t *T) GetStrings(s string) []string {
	k := t.key(s)
	return t.Config().GetStrings(k)
}

func (t *T) Tables() []string {
	return t.GetStrings("tables")
}

// AllowEmptyNetwork returns true if the driver supports
// empty "network" keyword value.
// For one, the loopback driver does support that.
func (t T) AllowEmptyNetwork() bool {
	return t.allowEmptyNetwork
}

func (t T) SetAllowEmptyNetwork(v bool) {
	t.allowEmptyNetwork = v
}

func (t T) IsIP6() bool {
	ip, _, err := net.ParseCIDR(t.Network())
	if err != nil {
		return false
	}
	return ip.To4() == nil
}

func (t *T) Network() string {
	return t.network
}

func (t *T) IPsPerNode() (int, error) {
	i, err := t.Config().Eval(cKey(t.Name(), "ips_per_node"))
	if err != nil {
		return 0, err
	}
	return i.(int), nil
}

func (t *T) SetNetwork(s string) {
	t.network = s
}

func (t *T) SetImplicit(v bool) {
	t.isImplicit = v
}

func (t T) IsImplicit() bool {
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

func (t *T) NodeSubnetIP(nodename string) (net.IP, error) {
	subnet, err := t.NodeSubnet(nodename)
	if err != nil {
		return nil, err
	}
	ip := subnet.IP
	IncIPN(ip, 1)
	return ip, nil
}

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
//
//	cluster.nodes = n1 n2 n3
//	net1.network = 10.0.0.0/24
//	net1.ips_per_node = 64
//
// =>
//
//	subnet@n1 = 10.0.0.0/26
//	subnet@n2 = 10.0.0.64/26
//	subnet@n3 = 10.0.0.128/26
func (t *T) NodeSubnet(nodename string) (*net.IPNet, error) {
	if nodename == "" {
		return nil, fmt.Errorf("empty nodename")
	}
	if subnet := t.GetSubnetAs(nodename); subnet == "" {
		// no configured subnet yet => allocate one
	} else if _, ipnet, err := net.ParseCIDR(subnet); err != nil {
		return nil, err
	} else if ipnet != nil {
		t.Log().Debugf("node %s subnet %s read from config", nodename, ipnet)
		return ipnet, nil
	}

	nodes, err := t.Nodes()
	if err != nil {
		return nil, err
	}
	idx := stringslice.Index(nodename, nodes)
	ipsPerNode, err := t.IPsPerNode()
	if err != nil {
		return nil, err
	}
	ipsPerNode = 1 << bits.Len(uint(ipsPerNode)-1)
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
		t.Log().Warnf("assign subnet %s to node %s: %s", subnetIPNet, nodename, err)
	} else {
		t.Log().Infof("assign subnet %s to node %s", subnetIPNet, nodename)
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
		clusterStatus clusterdump.Data
	)
	b, err = c.NewGetClusterStatus().
		SetSelector(selector).
		Get()
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
		p := NewFromNoder(name, noder)
		if p == nil {
			continue
		}
		if p.Type() == "lo" {
			hasLO = true
		}
		if p.Name() == "default" {
			hasDefault = true
		}
		switch p.Network() {
		case "none", "None":
			// used to disable the default network
			p.Log().Infof("skip, disabled by network=none")
			continue
		}
		l = append(l, p)
	}
	if !hasLO {
		p := NewTyped("lo", "lo", "127.0.0.1/32", noder)
		p.SetImplicit(true)
		l = append(l, p)
	}
	if !hasDefault {
		p := NewTyped("default", "bridge", "10.22.0.0/16", noder)
		p.SetImplicit(true)
		l = append(l, p)
	}
	return l
}
