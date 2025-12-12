// Package clusterhb retrieve hbconfer objects found in node/cluster config
package clusterhb

import (
	"strings"

	"github.com/opensvc/om3/v3/core/hbcfg"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	// T struct holds hbcfg.Confer retriever
	T struct {
		*object.Node
	}
)

func New() (*T, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	t := &T{
		Node: n,
	}
	return t, nil
}

// Hbs returns list of hbcfg.Confer objects from node/cluster config
func (t *T) Hbs() []hbcfg.Confer {
	l := make([]hbcfg.Confer, 0)
	config := t.MergedConfig()
	for _, name := range t.HbNames() {
		p := hbcfg.New(name, config)
		if p == nil {
			// no confer found from name
			continue
		}
		p.SetName(name)
		p.SetDriver(p.Type())
		p.SetConfig(config)
		l = append(l, p)
	}
	return l
}

// HbNames returns list hb names from node/cluster config
func (t *T) HbNames() []string {
	l := make([]string, 0)
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "hb#") {
			continue
		}
		l = append(l, s)
	}
	return l
}
