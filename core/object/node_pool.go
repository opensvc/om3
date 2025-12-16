package object

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/drivers/pooldirectory"
	"github.com/opensvc/om3/v3/drivers/poolshm"
)

func (t *Node) ShowPoolsByName(ctx context.Context, name string) pool.StatusList {
	l := pool.NewStatusList()
	for _, p := range t.Pools() {
		if name != "" && name != p.Name() {
			continue
		}
		l = l.Add(ctx, p, true)
	}
	return l
}

func (t *Node) ShowPools(ctx context.Context) pool.StatusList {
	l := pool.NewStatusList()
	for _, p := range t.Pools() {
		l = l.Add(ctx, p, true)
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
	var hasSHM, hasDefault bool
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "pool#") {
			continue
		}
		name := s[5:]
		if name == "shm" {
			hasSHM = true
		} else if name == "default" {
			hasDefault = true
		}
		l = append(l, s[5:])
	}
	if !hasSHM {
		l = append(l, "shm")
	}
	if !hasDefault {
		l = append(l, "default")
	}
	return l
}
