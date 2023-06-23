package clusterip

import (
	"net"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/render/tree"
)

type (
	T struct {
		IP   net.IP `json:"ip" yaml:"ip"`
		Node string `json:"node" yaml:"node"`
		Path path.T `json:"path" yaml:"path"`
		RID  string `json:"rid" yaml:"rid"`
	}
	L []T
)

func NewL() L {
	return make(L, 0)
}

func (t L) ByNetwork(n *net.IPNet) L {
	if n == nil {
		return t
	}
	l := NewL()
	for _, e := range t {
		if !n.Contains(e.IP) {
			continue
		}
		l = append(l, e)
	}
	return l
}

func (t L) Len() int {
	return len(t)
}

func (t L) Less(i, j int) bool {
	return t[i].Path.String() < t[j].Path.String()
}

func (t L) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t L) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText("ip").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("node").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("object").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("resource").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
	sort.Sort(t)
	for _, data := range t {
		n := head.AddNode()
		data.LoadTreeNode(n)
	}
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t T) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.IP.String())
	head.AddColumn().AddText(t.Node)
	head.AddColumn().AddText(t.Path.String())
	head.AddColumn().AddText(t.RID)
	head.AddColumn().AddText("")
	head.AddColumn().AddText("")
	head.AddColumn().AddText("")
}

func (t L) Load(clusterStatus cluster.Data) L {
	l := NewL()
	for nodename, nodeData := range clusterStatus.Cluster.Node {
		for ps, inst := range nodeData.Instance {
			if inst.Status == nil {
				continue
			}
			for _, resourceData := range inst.Status.Resources {
				rid := resourceData.Rid
				if ipIntf, ok := resourceData.Info["ipaddr"]; ok {
					p, err := path.Parse(ps)
					if err != nil {
						log.Debug().Err(err).Str("path", ps).Send()
						continue
					}
					ip := T{
						IP:   net.ParseIP(ipIntf.(string)),
						Path: p,
						Node: nodename,
						RID:  rid,
					}
					l = append(l, ip)
				}
			}
		}
	}
	return l
}
