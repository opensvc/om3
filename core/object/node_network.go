package object

import (
	"strings"

	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/drivers/networkbridge"
	"opensvc.com/opensvc/drivers/networklo"
)

func (t *Node) ShowNetworksByName(name string) network.StatusList {
	l := network.NewStatusList()
	for _, p := range t.Networks() {
		if name != "" && name != p.Name() {
			continue
		}
		l = l.Add(p, true)
	}
	return l
}

func (t *Node) ShowNetworks() network.StatusList {
	l := network.NewStatusList()
	for _, p := range t.Networks() {
		l = l.Add(p, true)
	}
	return l
}

func (t *Node) Networks() []network.Networker {
	l := make([]network.Networker, 0)
	config := t.MergedConfig()
	hasLO := false
	hasDefault := false

	for _, name := range t.ListNetworks() {
		p := network.New(name, config)
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
		p := networklo.NewNetworker()
		p.SetName("lo")
		p.SetDriver("lo")
		p.SetConfig(config)
		l = append(l, p)
	}
	if !hasDefault {
		p := networkbridge.NewNetworker()
		p.SetName("default")
		p.SetDriver("bridge")
		p.SetConfig(config)
		l = append(l, p)
	}
	return l
}

func (t *Node) ListNetworks() []string {
	l := make([]string, 0)
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "network#") {
			continue
		}
		l = append(l, s[8:])
	}
	return l
}
