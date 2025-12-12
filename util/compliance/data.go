package compliance

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/hostname"
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

func (t T) GetObjectData(p naming.Path, modsets []string) (Data, error) {
	data := Data{}
	err := t.collectorClient.CallFor(&data, "comp_get_svc_data_v2", hostname.Hostname(), p.String(), modsets)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (t T) GetData(modsets []string) (Data, error) {
	if t.objectPath.IsZero() {
		return t.GetNodeData(modsets)
	} else {
		return t.GetObjectData(t.objectPath, modsets)
	}
}

func (t T) GetNodeData(modsets []string) (Data, error) {
	data := Data{}
	err := t.collectorClient.CallFor(&data, "comp_get_data_v2", hostname.Hostname(), modsets)
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

// HeadModulesets returns the name of modulesets that either
//   - have no parent in ModsetRelations
//   - have a parent in ModsetRelations, but this parent is not in Modsets,
//     ie not attached
func (t Data) HeadModulesets() []string {
	l := make([]string, 0)
	m := t.ModsetRelations.Parents()
	hasParent := func(name string) bool {
		parents, ok := m[name]
		if !ok {
			return false
		}
		for _, parent := range parents {
			if _, ok := t.Modsets[parent]; ok {
				return true
			}
		}
		return false
	}
	for name := range t.Modsets {
		if !hasParent(name) {
			l = append(l, name)
		}
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

func (t Data) CascadingModulesetModules(name string) map[string]string {
	m := make(map[string]string)
	modsetRelations, ok := t.ModsetRelations[name]
	if ok {
		for _, child := range modsetRelations {
			// recurse for child modset
			for modName, modsetName := range t.CascadingModulesetModules(child) {
				m[modName] = modsetName
			}
		}
	}
	modset, ok := t.Modsets[name]
	if ok {
		for _, modName := range modset.ModuleNames() {
			m[modName] = name
		}
	}
	return m
}

func (t Data) AllModulesMap() map[string]*Module {
	m := make(map[string]*Module)
	for modsetName, mods := range t.Modsets {
		for _, mod := range mods {
			module := NewModule(mod.Name)
			module.SetAutofix(mod.AutoFix)
			module.SetModulesetName(modsetName)
			m[mod.Name] = module
		}
	}
	return m
}

func (t Data) AllModules() Modules {
	l := make(Modules, 0)
	m := t.AllModulesMap()
	for _, module := range m {
		l = append(l, module)
	}
	return l
}

// ExpandModules returns a map indexed by module name, with the
// hosting moduleset name as value.
func (t Data) ExpandModules(modsetNames, modNames []string) Modules {
	if len(modsetNames)+len(modNames) == 0 {
		return t.AllModules()
	}
	all := t.AllModulesMap()
	m := make(map[string]*Module)
	for _, modName := range modNames {
		if mod, ok := all[modName]; ok {
			m[modName] = mod
		}
	}
	for _, modsetName := range modsetNames {
		for childModsetName, childModName := range t.CascadingModulesetModules(modsetName) {
			m[childModName] = NewModule(childModName).SetModulesetName(childModsetName)
		}
	}
	l := make(Modules, 0)
	for _, module := range m {
		l = append(l, module)
	}
	return l
}

func (t Data) Ruleset(name string) Ruleset {
	if rset, ok := t.Rsets[name]; ok {
		return rset
	} else {
		return Ruleset{}
	}
}

func (t Data) RulesetsMD5() string {
	return t.Ruleset("osvc_collector").GetString("ruleset_md5")
}
