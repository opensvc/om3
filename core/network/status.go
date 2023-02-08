package network

import (
	"fmt"
	"math"
	"net"
	"sort"

	"github.com/opensvc/om3/core/clusterip"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/render/tree"
)

type (
	StatusUsage struct {
		Free int     `json:"free"`
		Used int     `json:"used"`
		Size int     `json:"size"`
		Pct  float64 `json:"pct"`
	}

	Status struct {
		Name    string      `json:"name"`
		Type    string      `json:"type"`
		Network string      `json:"network"`
		IPs     clusterip.L `json:"ips"`
		Errors  []string    `json:"errors,omitempty"`
		StatusUsage
	}
	StatusList []Status
)

func NewStatus() Status {
	t := Status{}
	t.IPs = make(clusterip.L, 0)
	t.Errors = make([]string, 0)
	return t
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
		if _, n, err := net.ParseCIDR(data.Network); err == nil {
			ones, _ := n.Mask.Size()
			data.Size = int(math.Pow(2.0, float64(ones)))
		}
		data.Free = usage.Free
		data.Used = usage.Used
		if usage.Size == 0 {
			data.Pct = 100.0
		} else {
			data.Pct = float64(usage.Used) / float64(usage.Size) * 100.0
		}
	}
	return data
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
	head.AddColumn().AddText("name").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("type").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("network").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("size").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("used").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("free").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("pct").SetColor(rawconfig.Color.Bold)
	sort.Sort(t)
	for _, data := range t {
		n := head.AddNode()
		data.LoadTreeNode(n)
	}
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t Status) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Name).SetColor(rawconfig.Color.Primary)
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
		t.IPs.LoadTreeNode(n)
	}
}

func ShowNetworksByName(noder Noder, name string) StatusList {
	l := NewStatusList()
	for _, p := range Networks(noder) {
		if name != "" && name != p.Name() {
			continue
		}
		l = l.Add(p, true)
	}
	return l
}

func ShowNetworks(noder Noder) StatusList {
	return ShowNetworksByName(noder, "")
}
