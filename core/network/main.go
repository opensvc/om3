package network

import (
	"fmt"
	"sort"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/render/tree"
)

type (
	T struct {
		driver string
		name   string
		config *xconfig.T
	}

	StatusUsage struct {
		Free int     `json:"free"`
		Used int     `json:"used"`
		Size int     `json:"size"`
		Pct  float64 `json:"pct"`
	}

	Status struct {
		Name    string       `json:"name"`
		Type    string       `json:"type"`
		Network string       `json:"network"`
		IPs     IPStatusList `json:"ips"`
		Errors  []string     `json:"errors,omitempty"`
		StatusUsage
	}
	StatusList []Status

	IPStatus struct {
		IP   string `json:"ip"`
		Node string `json:"node"`
		Path path.T `json:"path"`
		RID  string `json:"rid"`
	}
	IPStatusList []IPStatus

	Networker interface {
		SetName(string)
		SetDriver(string)
		Name() string
		Network() string
		Type() string
		Usage() (StatusUsage, error)
		SetConfig(*xconfig.T)
		Config() *xconfig.T
	}
)

var (
	drivers = make(map[string]func() Networker)
)

func NewStatus() Status {
	t := Status{}
	t.IPs = make(IPStatusList, 0)
	t.Errors = make([]string, 0)
	return t
}

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

func New(name string, config *xconfig.T) Networker {
	networkType := cString(config, name, "type")
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

func GetStatus(t Networker, withUsage bool) Status {
	data := NewStatus()
	data.Type = t.Type()
	data.Name = t.Name()
	data.Network = t.Network()
	if withUsage {
		usage, err := t.Usage()
		if err != nil {
			data.Errors = append(data.Errors, err.Error())
		}
		data.Free = usage.Free
		data.Used = usage.Used
		data.Size = usage.Size
		if usage.Size == 0 {
			data.Pct = 100.0
		} else {
			data.Pct = float64(usage.Used) / float64(usage.Size) * 100.0
		}
	}
	return data
}

func pKey(p Networker, s string) key.T {
	return key.New("network#"+p.Name(), s)
}

func (t *T) GetString(s string) string {
	k := key.New("network#"+t.name, s)
	return t.Config().GetString(k)
}

func (t *T) Network() string {
	return t.GetString("network")
}

func NewStatusList() StatusList {
	l := make(StatusList, 0)
	return StatusList(l)
}

func (t StatusList) Len() int {
	return len(t)
}

func (t StatusList) Less(i, j int) bool {
	return t[i].Name < t[j].Name
}

func (t StatusList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t StatusList) Add(p Networker, withUsage bool) StatusList {
	s := GetStatus(p, withUsage)
	l := []Status(t)
	l = append(l, s)
	return StatusList(l)
}

func (t StatusList) Render(verbose bool) string {
	nt := t
	if !verbose {
		for i, _ := range nt {
			nt[i].IPs = nil
		}
	}
	return nt.Tree().Render()
}

// Tree returns a tree loaded with the type instance.
func (t StatusList) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t StatusList) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText("name").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("type").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("network").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("size").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("used").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("free").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("pct").SetColor(rawconfig.Node.Color.Bold)
	sort.Sort(t)
	for _, data := range t {
		n := head.AddNode()
		data.LoadTreeNode(n)
	}
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t Status) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Name).SetColor(rawconfig.Node.Color.Primary)
	head.AddColumn().AddText(t.Type)
	head.AddColumn().AddText(t.Network)
	if t.Size == 0 {
		head.AddColumn().AddText("-")
		head.AddColumn().AddText("-")
		head.AddColumn().AddText("-")
		head.AddColumn().AddText("-")
	} else {
		head.AddColumn().AddText(fmt.Sprint(t.Size))
		head.AddColumn().AddText(fmt.Sprint(t.Used))
		head.AddColumn().AddText(fmt.Sprint(t.Free))
		head.AddColumn().AddText(fmt.Sprintf("%.2f%%", t.Pct))
	}
	if len(t.IPs) > 0 {
		n := head.AddNode()
		IPStatusList(t.IPs).LoadTreeNode(n)
	}
}

func (t IPStatusList) Len() int {
	return len(t)
}

func (t IPStatusList) Less(i, j int) bool {
	return t[i].Path.String() < t[j].Path.String()
}

func (t IPStatusList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t IPStatusList) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText("ip").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("node").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("object").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("resource").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	sort.Sort(t)
	for _, data := range t {
		n := head.AddNode()
		data.LoadTreeNode(n)
	}
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t IPStatus) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.IP)
	head.AddColumn().AddText(t.Node)
	head.AddColumn().AddText(t.Path.String())
	head.AddColumn().AddText(t.RID)
	head.AddColumn().AddText("")
	head.AddColumn().AddText("")
	head.AddColumn().AddText("")
}
