package object

import (
	"strings"

	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/drivers/poolshm"
)

func (t *Node) ShowPools() pool.StatusList {
	l := pool.NewStatusList()
	for _, p := range t.Pools() {
		l = l.Add(p)
	}
	return l
}

func (t *Node) Pools() []pool.Pooler {
	l := make([]pool.Pooler, 0)
	config := t.MergedConfig()

	p := poolshm.NewPooler("shm")
	p.SetConfig(t.MergedConfig())
	l = append(l, p)

	for _, name := range t.ListPools() {
		p := pool.New(name, config)
		if p == nil {
			continue
		}
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
