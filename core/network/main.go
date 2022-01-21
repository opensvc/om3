package network

import (
	"encoding/json"
	"net"
	"sort"
	"strings"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/clusterip"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
)

type (
	T struct {
		driver     string
		name       string
		isImplicit bool
		config     *xconfig.T
	}

	Networker interface {
		SetName(string)
		SetDriver(string)
		SetImplicit()
		IsImplicit() bool
		IsValid() bool
		Name() string
		Network() string
		Type() string
		Usage() (StatusUsage, error)
		SetConfig(*xconfig.T)
		Config() *xconfig.T
		FilterIPs(clusterip.L) clusterip.L
		AllowEmptyNetwork() bool
	}
	Setuper interface {
		Setup(*object.Node) error
	}
	CNIer interface {
		CNIConfigData() (interface{}, error)
	}
)

var (
	drivers = make(map[string]func() Networker)
)

func sectionName(networkName string) string {
	return "network#" + networkName
}

func cKey(networkName string, option string) key.T {
	section := sectionName(networkName)
	return key.New(section, option)
}

func cString(config *xconfig.T, networkName string, option string) string {
	network := cKey(networkName, option)
	return config.GetString(network)
}

func NewTyped(name string, networkType string, config *xconfig.T) Networker {
	fn, ok := drivers[networkType]
	if !ok {
		return nil
	}
	t := fn()
	t.SetName(name)
	t.SetDriver(networkType)
	t.SetConfig(config)
	return t.(Networker)
}

func New(name string, config *xconfig.T) Networker {
	networkType := cString(config, name, "type")
	return NewTyped(name, networkType, config)
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

func (t *T) SetDriver(driver string) {
	t.driver = driver
}

func (t T) Type() string {
	return t.driver
}

func (t *T) Config() *xconfig.T {
	return t.config
}

func (t *T) SetConfig(c *xconfig.T) {
	t.config = c
}

func (t T) FilterIPs(ips clusterip.L) clusterip.L {
	l := make(clusterip.L, 0)
	_, n, err := net.ParseCIDR(t.Network())
	if err != nil {
		return l
	}
	return ips.ByNetwork(n)
}

func pKey(p Networker, s string) key.T {
	return key.New("network#"+p.Name(), s)
}

func (t *T) GetString(s string) string {
	k := key.New("network#"+t.name, s)
	return t.Config().GetString(k)
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

func (t *T) Network() string {
	return t.GetString("network")
}

func (t *T) SetImplicit() {
	t.isImplicit = true
}

func (t *T) IsImplicit() bool {
	return t.isImplicit
}

func Networks(n *object.Node) []Networker {
	l := make([]Networker, 0)
	config := n.MergedConfig()
	hasLO := false
	hasDefault := false

	for _, name := range namesInConfig(n) {
		p := New(name, config)
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
		p := NewTyped("lo", "lo", config)
		p.SetImplicit()
		l = append(l, p)
	}
	if !hasDefault {
		p := NewTyped("default", "bridge", config)
		p.SetImplicit()
		l = append(l, p)
	}
	return l
}

func List(n *object.Node) []string {
	l := make([]string, 0)
	for _, n := range Networks(n) {
		l = append(l, n.Name())
	}
	sort.Strings(l)
	return l
}

func namesInConfig(n *object.Node) []string {
	l := make([]string, 0)
	for _, s := range n.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "network#") {
			continue
		}
		l = append(l, s[8:])
	}
	return l
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
