package clusterip

import (
	"net"
	"sort"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/render/tree"
)

type (
	T struct {
		IP   net.IP `json:"ip"`
		Node string `json:"node"`
		Path path.T `json:"path"`
		RID  string `json:"rid"`
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

func (t L) Load(clusterStatus cluster.Status) L {
	l := NewL()
	for nodename, nodeData := range clusterStatus.Cluster.Node {
		for ps, instanceData := range nodeData.Services.Status {
			for rid, resourceData := range instanceData.Resources {
				if ipIntf, ok := resourceData.Info["ipaddr"]; ok {
					p, err := path.Parse(ps)
					if err != nil {
						log.Debug().Err(err).Str("path", ps).Msg("")
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
