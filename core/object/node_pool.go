package object

import (
	"strings"

	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/drivers/pooldirectory"
	"opensvc.com/opensvc/drivers/poolshm"
)

func (t *Node) ShowPoolsByName(name string) pool.StatusList {
	l := pool.NewStatusList()
	for _, p := range t.Pools() {
		if name != "" && name != p.Name() {
			continue
		}
		l = l.Add(p, true)
	}
	return l
}

func (t *Node) ShowPools() pool.StatusList {
	l := pool.NewStatusList()
	for _, p := range t.Pools() {
		l = l.Add(p, true)
	}
	return l
}

func (t *Node) Pools() []pool.Pooler {
	l := make([]pool.Pooler, 0)
	config := t.MergedConfig()
	hasSHM := false
	hasDefault := false

	for _, name := range t.ListPools() {
		p := pool.New(name, config)
		if p == nil {
			continue
		}
		if p.Type() == "shm" {
			hasSHM = true
		}
		if p.Name() == "default" {
			hasDefault = true
		}
		l = append(l, p)
	}
	if !hasSHM {
		p := poolshm.NewPooler()
		p.SetName("shm")
		p.SetDriver("shm")
		p.SetConfig(config)
		l = append(l, p)
	}
	if !hasDefault {
		p := pooldirectory.NewPooler()
		p.SetName("default")
		p.SetDriver("directory")
		p.SetConfig(config)
		l = append(l, p)
	}
	return l
}

func (t *Node) ListPools() []string {
	l := make([]string, 0)
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "pool#") {
			continue
		}
		l = append(l, s[5:])
	}
	return l
}
