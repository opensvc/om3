package object

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/core/pool"
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
		if p := implicitPool("shm", "shm", config); p != nil {
			l = append(l, p)
		}
	}
	if !hasDefault {
		if p := implicitPool("default", "directory", config); p != nil {
			l = append(l, p)
		}
	}
	return l
}

// implicitPool returns a pool the node has with no configuration section,
// allocated by the driver registered for its type, and nil when the binary
// links no such driver.
func implicitPool(name, poolType string, config pool.Config) pool.Pooler {
	fn := pool.Driver(poolType)
	if fn == nil {
		return nil
	}
	p := fn()
	p.SetName(name)
	p.SetDriver(poolType)
	p.SetConfig(config)
	return p
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
