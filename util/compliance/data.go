package compliance

import (
	"fmt"
	"sort"
	"strings"

	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/xmap"
)

type (
	Data struct {
		Modsets             Modulesets                `json:"modulesets,omitempty"`
		Rsets               Rulesets                  `json:"rulesets,omitempty"`
		ModsetRsetRelations ModulesetRulesetRelations `json:"modset_rset_relations,omitempty"`
		ModsetRelations     ModulesetRelations        `json:"modset_relations,omitempty"`
	}
	ModulesetTree struct {
		Name       string
		Modules    []ModulesetModule
		Modulesets []*ModulesetTree
	}
)

func renderModulesetTree(prefixLen int, modset ModulesetTree) string {
	buff := ""
	if prefixLen >= 0 {
		prefix := strings.Repeat(" ", prefixLen)
		buff += prefix + modset.Name + "\n"
		for _, mod := range modset.Modules {
			buff += fmt.Sprintf("%s %s\n", prefix, mod.Render())
		}
	}
	for _, child := range modset.Modulesets {
		buff += renderModulesetTree(prefixLen+1, *child)
	}
	return buff
}

func (t ModulesetTree) Render() string {
	return renderModulesetTree(-1, t)
}

func (t *ModulesetTree) AddModuleset(data Data, modset string) {
	node := &ModulesetTree{
		Name:       modset,
		Modules:    data.Modsets.ModulesOf(modset),
		Modulesets: []*ModulesetTree{},
	}
	t.Modulesets = append(t.Modulesets, node)
	rels, ok := data.ModsetRelations[modset]
	if !ok {
		return
	}
	for _, rel := range rels {
		node.AddModuleset(data, rel)
	}
}

func (t T) GetAllData(modsets []string) (Data, error) {
	return t.GetData([]string{})
}

func (t T) GetData(modsets []string) (Data, error) {
	data := Data{}
	err := t.collectorClient.CallFor(&data, "comp_get_data", hostname.Hostname(), modsets)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (t Data) Render() string {
	return fmt.Sprintf("%s\n%s\n%s\n%s\n",
		t.Rsets,
		t.Modsets,
		t.ModsetRelations,
		t.ModsetRsetRelations,
	)
}

func (t ModulesetRelations) Parents() map[string][]string {
	m := make(map[string][]string)
	for parent, children := range t {
		for _, child := range children {
			if _, ok := m[child]; ok {
				m[child] = append(m[child], parent)
			} else {
				m[child] = []string{parent}
			}
		}
	}
	return m
}

func (t Data) HeadModulesets() []string {
	l := make([]string, 0)
	m := t.ModsetRelations.Parents()
	for name, _ := range t.Modsets {
		if _, ok := m[name]; ok {
			continue
		}
		l = append(l, name)
	}
	return l
}

func (t Data) ModulesetTree(modset string) *ModulesetTree {
	tree := &ModulesetTree{}
	tree.AddModuleset(t, modset)
	return tree
}

func (t Data) ModulesetsTree() *ModulesetTree {
	tree := &ModulesetTree{}
	for _, head := range t.HeadModulesets() {
		tree.AddModuleset(t, head)
	}
	return tree
}

func (t Data) CascadingModulesetModules(name string) []string {
	l := make([]string, 0)
	modsetRelations, ok := t.ModsetRelations[name]
	if ok {
		for _, child := range modsetRelations {
			// recurse for child modset
			l = append(l, t.CascadingModulesetModules(child)...)
		}
	}
	modset, ok := t.Modsets[name]
	if ok {
		l = append(l, modset.ModuleNames()...)
	}
	return l
}

func (t Data) AllModuleNames() []string {
	m := make(map[string]interface{})
	for _, mods := range t.Modsets {
		for _, mod := range mods {
			m[mod.Name] = nil
		}
	}
	l := xmap.Keys(m)
	sort.Strings(l)
	return l
}

func (t Data) ExpandModules(modsets, mods []string) []string {
	m := make(map[string]interface{})
	if len(modsets)+len(mods) == 0 {
		return t.AllModuleNames()
	}
	for _, mod := range mods {
		m[mod] = nil
	}
	for _, modset := range modsets {
		for _, mod := range t.CascadingModulesetModules(modset) {
			m[mod] = nil
		}
	}
	l := xmap.Keys(m)
	sort.Strings(l)
	return l
}
