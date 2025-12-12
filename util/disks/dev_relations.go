package disks

import (
	"github.com/opensvc/om3/v3/util/command"
)

type (
	//
	// Relation links 2 Dev in Parent-Child relationship, in the sense
	// of block device stacking done by drivers such as dm or md.
	// For example:
	//
	// sda
	// `-sda1   <= child of sda, parent of lv1
	//   `-lv1
	//
	Relation struct {
		Child  string
		Parent string
	}

	Relations map[string]map[string]Relation
)

var (
	_relations Relations
)

// LeavesOf return the device names of block devices at the end of
// each branch of lsblk, starting at "parent".
func (t Relations) LeavesOf(parent string) []string {
	l := make([]string, 0)
	for k := range t.leavesOf(parent) {
		l = append(l, k)
	}
	return l
}

// Use a map[string]interface{} to avoid dup leaves
// For example: raid legs from the same disk)
func (t Relations) leavesOf(parent string) map[string]interface{} {
	m := make(map[string]interface{})
	pm, ok := t[parent]
	if !ok || len(pm) == 0 {
		m[parent] = nil
		return m
	}
	for child := range pm {
		for name := range t.leavesOf(child) {
			m[name] = nil
		}
	}
	return m
}

// rootOf returns the topmost parent of the leaf
// For example,
//
//	chain:       sda > md127 > lv1
//	rootOf(lv1): sda
func (t Relations) rootOf(leaf string) string {
	if d, ok := _devices[leaf]; ok {
		if d.Type == "mpath" {
			return leaf
		}
	}
	for parent, pm := range t {
		if parent == "" {
			continue
		}
		for child := range pm {
			if child == leaf {
				return t.rootOf(parent)
			}
		}
	}
	return leaf
}

// leafOf returns true if nodeA is a leaf of NodeB.
// Example:
//
//	chain:       sda > md127 > lv1 > lv2
//	leafOf(lv2, lv1):   true
//	leafOf(lv2, md127): true
//	leafOf(md127, lv2): false
//
// This is useful to merge claims.
func (t Relations) leafOf(nodeA, nodeB string) bool {
	if nodeB == "" {
		return false
	}
	pm, ok := t[nodeB]
	if !ok {
		return false
	}
	for child := range pm {
		if child == nodeA {
			return true
		}
		if t.leafOf(nodeA, child) {
			return true
		}
	}
	return false
}

func loadRelations() error {
	_relations = make(Relations)
	chain := make([]string, 0)
	parse := func(line string) {
		for _, pair := range relRE.FindAllStringSubmatch(line, -1) {
			depth := len(pair[1])/2 + 1
			name := pair[2]
			rel := Relation{
				Child: name,
			}
			chainLen := len(chain)
			switch {
			case chainLen == 0:
				// sda
				rel.Parent = ""
			case depth > chainLen:
				// sda
				// `-sda1
				rel.Parent = chain[len(chain)-1]
			case depth == 1:
				// sda
				// `-sda1
				// sdb
				chain = chain[0:0]
				rel.Parent = ""
			default:
				// sda
				// |-sda1
				// | `- lv
				// `-sdb
				chain = chain[0 : depth-1]
				rel.Parent = chain[len(chain)-1]
			}
			chain = append(chain, name)
			if _, ok := _relations[rel.Parent]; !ok {
				_relations[rel.Parent] = make(map[string]Relation)
			}
			_relations[rel.Parent][rel.Child] = rel
		}
	}
	cmd := command.New(
		command.WithName("lsblk"),
		command.WithVarArgs("-o", "NAME", "--ascii", "-e7", "-n"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
