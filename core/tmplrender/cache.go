package tmplrender

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/xerrors"
)

// Item is a thing a template reads from the cluster data: an object, an
// instance, a node, a resource, a network, by the json names of its fields,
// in snake case, as the api names them.
type Item = map[string]any

// ipInfoKeys are the keys of the information of an ip resource an ips item
// has as fields of its own.
var ipInfoKeys = []string{"ipaddr", "dev", "netmask", "expose", "hostname"}

// isVolatile returns true for the fields of the cluster data changing at each
// refresh, as a status refreshed with no change: a template reading them would
// be rendered again at each refresh.
func isVolatile(k string) bool {
	switch k {
	case "updated_at", "outdated_at", "log", "monitor_action_executed_at":
		return true
	}
	return strings.HasSuffix(k, "_updated_at")
}

// asItem returns v by the json names of its fields, with no volatile field.
func asItem(v any) (Item, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := make(Item)
	if string(b) == "null" {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	dropVolatile(m)
	return m, nil
}

func dropVolatile(v any) {
	switch m := v.(type) {
	case map[string]any:
		for k, e := range m {
			if isVolatile(k) {
				delete(m, k)
				continue
			}
			dropVolatile(e)
		}
	case []any:
		for _, e := range m {
			dropVolatile(e)
		}
	}
}

// selectPaths returns the paths among paths the object selector sel selects,
// all of them when sel is empty. Only the path terms of the selector syntax
// are allowed: a term about the configuration, as app=foo, needs the
// configuration files, a client does not have.
func selectPaths(sel string, paths []string) (map[string]bool, error) {
	m := make(map[string]bool)
	if sel == "" {
		for _, s := range paths {
			m[s] = true
		}
		return m, nil
	}
	l := make(naming.Paths, 0, len(paths))
	for _, s := range paths {
		if p, err := naming.ParsePath(s); err == nil {
			l = append(l, p)
		}
	}
	selection := objectselector.New(sel,
		objectselector.WithPaths(l),
		objectselector.WithLocal(true),
		objectselector.WithConfigFilterDisabled(),
	)
	if err := selection.CheckFilters(); err != nil {
		return nil, err
	}
	selected, err := selection.Expand()
	if err != nil && !errors.Is(err, xerrors.ObjectNotFound) {
		return nil, err
	}
	for _, p := range selected {
		m[p.String()] = true
	}
	return m, nil
}

// nodeSelected returns true when the node name with labels is one of the
// nodes the selector sel selects, terms separated by commas, each a node
// name pattern or a label=value: all of them when sel is empty.
func nodeSelected(sel, name string, labels map[string]string) bool {
	if sel == "" {
		return true
	}
	for _, term := range strings.Split(sel, ",") {
		if k, v, ok := strings.Cut(term, "="); ok {
			if got, ok := labels[k]; ok && fnmatch.Match(v, got, 0) {
				return true
			}
			continue
		}
		if fnmatch.Match(term, name, 0) {
			return true
		}
	}
	return false
}

func sortedNodes(data *clusterdump.Data) []string {
	l := make([]string, 0, len(data.Cluster.Node))
	for k := range data.Cluster.Node {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}

// objectPaths returns the paths of the objects with a status, and of the
// instances of all the nodes.
func objectPaths(data *clusterdump.Data) []string {
	m := make(map[string]bool)
	for p := range data.Cluster.Object {
		m[p] = true
	}
	for _, n := range data.Cluster.Node {
		for p := range n.Instance {
			m[p] = true
		}
	}
	return sortedKeys(m)
}

func pathFields(item Item, s string) {
	item["path"] = s
	if p, err := naming.ParsePath(s); err == nil {
		item["namespace"] = p.Namespace
		item["kind"] = p.Kind.String()
		item["name"] = p.Name
	}
}

// objects returns the objects the selector sel selects, sorted by path.
func objects(data *clusterdump.Data, sel string) ([]Item, error) {
	paths := make([]string, 0, len(data.Cluster.Object))
	for p := range data.Cluster.Object {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	selected, err := selectPaths(sel, paths)
	if err != nil {
		return nil, err
	}
	l := make([]Item, 0)
	for _, s := range paths {
		if !selected[s] {
			continue
		}
		item, err := asItem(data.Cluster.Object[s])
		if err != nil {
			return nil, err
		}
		pathFields(item, s)
		l = append(l, item)
	}
	return l, nil
}

// object returns the object of path, an error when it has no status.
func objectItem(data *clusterdump.Data, path string) (Item, error) {
	o, ok := data.Cluster.Object[path]
	if !ok {
		return nil, fmt.Errorf("object %s: %w", path, ErrNotFound)
	}
	item, err := asItem(o)
	if err != nil {
		return nil, err
	}
	pathFields(item, path)
	return item, nil
}

// instances returns the instances of the objects the selector sel selects,
// sorted by path and node. An instance has its status, its scope, labels,
// app and env from its configuration, and its monitor as the monitor field.
// The resources are read with the resources function.
func instances(data *clusterdump.Data, sel string) ([]Item, error) {
	selected, err := selectPaths(sel, objectPaths(data))
	if err != nil {
		return nil, err
	}
	l := make([]Item, 0)
	for _, node := range sortedNodes(data) {
		n := data.Cluster.Node[node]
		paths := make([]string, 0, len(n.Instance))
		for p := range n.Instance {
			if selected[p] {
				paths = append(paths, p)
			}
		}
		sort.Strings(paths)
		for _, p := range paths {
			item, err := instanceItem(data, p, node)
			if err != nil {
				return nil, err
			}
			if item != nil {
				l = append(l, item)
			}
		}
	}
	sort.SliceStable(l, func(i, j int) bool {
		return l[i]["path"].(string) < l[j]["path"].(string)
	})
	return l, nil
}

// instanceItem returns the instance of path on node, nil when the node has
// none.
func instanceItem(data *clusterdump.Data, path, node string) (Item, error) {
	n, ok := data.Cluster.Node[node]
	if !ok {
		return nil, nil
	}
	inst, ok := n.Instance[path]
	if !ok || (inst.Status == nil && inst.Config == nil) {
		return nil, nil
	}
	item := make(Item)
	if inst.Status != nil {
		var err error
		if item, err = asItem(inst.Status); err != nil {
			return nil, err
		}
		delete(item, "resources")
	}
	if inst.Config != nil {
		item["scope"] = inst.Config.Scope
		item["labels"] = map[string]string(inst.Config.Labels)
		if inst.Config.ActorConfig != nil {
			item["app"] = inst.Config.App
			item["env"] = inst.Config.Env
		}
	}
	if inst.Monitor != nil {
		mon, err := asItem(inst.Monitor)
		if err != nil {
			return nil, err
		}
		item["monitor"] = mon
	}
	pathFields(item, path)
	item["node"] = node
	return item, nil
}

// resources returns the resources of the instances of the objects the
// selector sel selects, of the driver group group, or of any group when it is
// zero, whose id matches the pattern rid, or of any id when it is empty,
// sorted by path, node and id.
func resources(data *clusterdump.Data, sel string, group driver.Group, rid string) ([]Item, error) {
	selected, err := selectPaths(sel, objectPaths(data))
	if err != nil {
		return nil, err
	}
	l := make([]Item, 0)
	for _, node := range sortedNodes(data) {
		for p, inst := range data.Cluster.Node[node].Instance {
			if !selected[p] || inst.Status == nil {
				continue
			}
			for id, rst := range inst.Status.Resources {
				if rid != "" && !fnmatch.Match(rid, id, 0) {
					continue
				}
				if group != 0 && driver.Parse(rst.Type).Group != group {
					continue
				}
				item, err := asItem(rst)
				if err != nil {
					return nil, err
				}
				pathFields(item, p)
				item["node"] = node
				item["rid"] = id
				if group == driver.GroupIP {
					// Always set, empty when the driver tells
					// none, as an ip down: a template reads
					// them without checking they exist.
					for _, k := range ipInfoKeys {
						if v, ok := rst.Info[k]; ok {
							item[k] = v
						} else {
							item[k] = ""
						}
					}
				}
				l = append(l, item)
			}
		}
	}
	sort.Slice(l, func(i, j int) bool {
		a, b := l[i], l[j]
		if a["path"] != b["path"] {
			return a["path"].(string) < b["path"].(string)
		}
		if a["node"] != b["node"] {
			return a["node"].(string) < b["node"].(string)
		}
		return a["rid"].(string) < b["rid"].(string)
	})
	return l, nil
}

// nodes returns the nodes the node selector sel selects, sorted by name.
func nodes(data *clusterdump.Data, sel string) []Item {
	l := make([]Item, 0)
	for _, name := range sortedNodes(data) {
		n := data.Cluster.Node[name]
		if !nodeSelected(sel, name, n.Config.Labels) {
			continue
		}
		l = append(l, nodeItem(name, data))
	}
	return l
}

func nodeItem(name string, data *clusterdump.Data) Item {
	n := data.Cluster.Node[name]
	labels := map[string]string(n.Config.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	return Item{
		"name":          name,
		"labels":        labels,
		"env":           n.Config.Env,
		"agent":         n.Status.Agent,
		"booted_at":     n.Status.BootedAt,
		"frozen_at":     n.Status.FrozenAt,
		"frozen_scope":  n.Status.FrozenScope,
		"is_leader":     n.Status.IsLeader,
		"is_overloaded": n.Status.IsOverloaded,
	}
}

// networks returns the networks of the nodes, of the name pattern name, or of
// any name when it is empty, sorted by name and node.
func networks(data *clusterdump.Data, name string) ([]Item, error) {
	l := make([]Item, 0)
	for _, node := range sortedNodes(data) {
		for netName, st := range data.Cluster.Node[node].Network {
			if name != "" && !fnmatch.Match(name, netName, 0) {
				continue
			}
			item, err := asItem(st)
			if err != nil {
				return nil, err
			}
			item["name"] = netName
			item["node"] = node
			l = append(l, item)
		}
	}
	sort.Slice(l, func(i, j int) bool {
		a, b := l[i], l[j]
		if a["name"] != b["name"] {
			return a["name"].(string) < b["name"].(string)
		}
		return a["node"].(string) < b["node"].(string)
	})
	return l, nil
}

// configVersion returns what the evaluation of a keyword of the object path
// as node evaluates it depends on: the configuration of the object, of the
// node, and of the cluster.
func configVersion(data *clusterdump.Data, path, node string) string {
	return fingerprint(struct {
		Object  map[string]string
		Node    any
		Cluster any
	}{
		Object:  dataStoreChecksums(data, path),
		Node:    data.Cluster.Node[node].Config,
		Cluster: data.Cluster.Config,
	})
}

// dataStoreChecksums returns the checksums of the configuration of the object
// path, by node: one changes when a key of a datastore changes.
func dataStoreChecksums(data *clusterdump.Data, path string) map[string]string {
	m := make(map[string]string)
	for node, n := range data.Cluster.Node {
		if inst, ok := n.Instance[path]; ok && inst.Config != nil {
			m[node] = inst.Config.Checksum
		}
	}
	return m
}
