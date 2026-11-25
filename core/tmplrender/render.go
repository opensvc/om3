// Package tmplrender renders a Go template from the cluster data an events
// stream keeps, and from the values fetched from the api no event carries:
// the datastore keys, the evaluated keywords, the resource informations.
//
// A render records each read it makes, the events telling it may have
// changed, and its fingerprint: a watcher subscribes to these events, and
// renders again when a fingerprint changes. The reads are recorded as the
// template executes, so the ones under a condition, in a range, or of a
// computed name, are the ones the data led to.
package tmplrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"text/template"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/tmplfuncs"
)

type (
	// Renderer renders a template.
	Renderer struct {
		// Name names the template in its errors, as its file.
		Name string

		// Text is the template.
		Text string

		// Namespace is the namespace of the relative paths of the
		// template.
		Namespace string

		// Data is the data of the template, as .name reads it.
		Data map[string]any

		Fetcher Fetcher
	}

	// Result is a render.
	Result struct {
		// Out is the rendered template, nil when the render failed.
		Out []byte

		// Reads are the reads of the render, the ones made before an
		// error included: a template failing on a missing object
		// watches the object, and renders again when it appears.
		Reads []Read

		// ReadsSecret is true when the render read a sec key.
		ReadsSecret bool
	}

	// render is the state of one render.
	render struct {
		ctx       context.Context
		t         *Renderer
		st        *State
		reads     []Read
		readIDs   map[string]bool
		fetched   map[string]any
		readsSecr bool
	}
)

var (
	objectKinds   = []string{"ObjectStatusUpdated", "ObjectStatusDeleted"}
	instanceKinds = []string{
		"InstanceStatusUpdated", "InstanceStatusDeleted",
		"InstanceConfigUpdated", "InstanceConfigDeleted",
		"InstanceMonitorUpdated", "InstanceMonitorDeleted",
	}
	resourceKinds  = []string{"InstanceStatusUpdated", "InstanceStatusDeleted"}
	nodeKinds      = []string{"NodeConfigUpdated", "NodeStatusUpdated", "ForgetPeer"}
	networkKinds   = []string{"NodeNetworkStatusUpdated", "NodeNetworkStatusDeleted"}
	dataStoreKinds = []string{"InstanceConfigUpdated", "InstanceConfigDeleted"}
	configKinds    = []string{"InstanceConfigUpdated", "InstanceConfigDeleted", "NodeConfigUpdated", "ClusterConfigUpdated"}
	resInfoKinds   = []string{"InstanceResourceInfoUpdated"}
)

// Parse checks the template parses, and uses no unknown function.
func (t *Renderer) Parse() error {
	r := &render{t: t}
	_, err := r.parse()
	return err
}

// Render renders the template, reading the cluster data of st.
func (t *Renderer) Render(ctx context.Context, st *State) (Result, error) {
	r := &render{
		ctx:     ctx,
		t:       t,
		st:      st,
		readIDs: make(map[string]bool),
		fetched: make(map[string]any),
	}
	tmpl, err := r.parse()
	if err != nil {
		return Result{}, err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, t.Data)
	result := Result{Reads: r.reads, ReadsSecret: r.readsSecr}
	if err != nil {
		return result, err
	}
	result.Out = buf.Bytes()
	return result, nil
}

func (r *render) parse() (*template.Template, error) {
	funcs := tmplfuncs.FuncMap()
	for k, v := range r.funcs() {
		funcs[k] = v
	}
	return template.New(r.t.Name).Option("missingkey=error").Funcs(funcs).Parse(r.t.Text)
}

// track records a read, once by id, and returns the value it read. extract
// reads the value from a state, its fingerprint is the read's.
func track[T any](r *render, read Read, extract func(*State) (T, error)) (T, error) {
	fp := func(v T, err error) string {
		if err != nil {
			return "error:" + err.Error()
		}
		return fingerprint(v)
	}
	probe := func(st *State) string {
		return fp(extract(st))
	}
	v, err := extract(r.st)
	r.add(read, probe, fp(v, err))
	return v, err
}

// trackVersion records a read of a fetched value, whose fingerprint is the
// version of what it fetched.
func (r *render) trackVersion(read Read, version func(*State) string) {
	r.add(read, version, version(r.st))
}

func (r *render) add(read Read, probe func(*State) string, fp string) {
	if r.readIDs[read.ID] {
		return
	}
	r.readIDs[read.ID] = true
	read.probe = probe
	read.fingerprint = fp
	r.reads = append(r.reads, read)
}

// path returns the path s, relative to the namespace of the template.
func (r *render) path(s string) (naming.Path, error) {
	return naming.ParsePathRel(s, r.t.Namespace)
}

// dataStore returns the datastore path s, a cfg or a sec.
func (r *render) dataStore(s string) (naming.Path, error) {
	p, err := r.path(s)
	if err != nil {
		return p, err
	}
	switch p.Kind {
	case naming.KindCfg, naming.KindSec:
	default:
		return p, fmt.Errorf("%s is not a cfg or sec datastore", p)
	}
	return p, nil
}

// fetch returns the value fetch fetches, once per render by id.
func fetch[T any](r *render, id string, fetch func() (T, error)) (T, error) {
	if v, ok := r.fetched[id]; ok {
		if err, ok := v.(error); ok {
			var zero T
			return zero, err
		}
		return v.(T), nil
	}
	v, err := fetch()
	if err != nil {
		r.fetched[id] = err
	} else {
		r.fetched[id] = v
	}
	return v, err
}

func optional(args []string, i int) string {
	if len(args) > i {
		return args[i]
	}
	return ""
}

func (r *render) funcs() template.FuncMap {
	return template.FuncMap{
		// The datastore keys.
		"key":       r.key,
		"keyOr":     r.keyOr,
		"keyExists": r.keyExists,
		"keyNames":  r.keyNames,

		// The keywords, as a node evaluates them.
		"config":   r.config,
		"configAs": r.configAs,

		// The resource informations.
		"resinfo": r.resinfo,

		// The objects.
		"objects":      r.objects,
		"objectStatus": r.objectStatus,
		"objectExists": r.objectExists,
		"avail":        r.avail,
		"availOr":      r.availOr,

		// The instances and their resources.
		"instances":      r.instances,
		"instanceStatus": r.instanceStatus,
		"resources":      r.resources,
		"ips":            r.groupResources(driver.GroupIP),
		"disks":          r.groupResources(driver.GroupDisk),
		"apps":           r.groupResources(driver.GroupApp),

		// The nodes and networks.
		"nodes":     r.nodes,
		"nodeLabel": r.nodeLabel,
		"networks":  r.networks,
	}
}

// dataStoreRead records a read of the datastore p, whose version is the
// checksums of its configuration on the nodes.
func (r *render) dataStoreRead(id string, p naming.Path) {
	if p.Kind == naming.KindSec {
		r.readsSecr = true
	}
	s := p.String()
	r.trackVersion(Read{ID: id, Kinds: dataStoreKinds, Selection: s}, func(st *State) string {
		return fingerprint(dataStoreChecksums(st.Data, s))
	})
}

// dataStoreExists returns true when a node has an instance of the datastore
// p. The api answers no error for a key of a datastore no node has, but an
// empty value.
func (r *render) dataStoreExists(p naming.Path) bool {
	return len(dataStoreChecksums(r.st.Data, p.String())) > 0
}

func (r *render) fetchKey(p naming.Path, name string) (string, error) {
	if !r.dataStoreExists(p) {
		return "", ErrNotFound
	}
	b, err := fetch(r, "key "+p.String()+" "+name, func() ([]byte, error) {
		return r.t.Fetcher.DataKey(r.ctx, p, name)
	})
	return string(b), err
}

// key returns the value of the key name of the datastore path, an error when
// the datastore or the key does not exist.
func (r *render) key(path, name string) (string, error) {
	p, err := r.dataStore(path)
	if err != nil {
		return "", err
	}
	r.dataStoreRead("key "+p.String()+" "+name, p)
	s, err := r.fetchKey(p, name)
	if errors.Is(err, ErrNotFound) {
		if !r.dataStoreExists(p) {
			return "", fmt.Errorf("datastore %s: %w", p, ErrNotFound)
		}
		return "", fmt.Errorf("key %s of %s: %w", name, p, ErrNotFound)
	}
	return s, err
}

// keyOr returns the value of the key name of the datastore path, def when the
// datastore or the key does not exist.
func (r *render) keyOr(path, name, def string) (string, error) {
	p, err := r.dataStore(path)
	if err != nil {
		return "", err
	}
	r.dataStoreRead("key "+p.String()+" "+name, p)
	s, err := r.fetchKey(p, name)
	if errors.Is(err, ErrNotFound) {
		return def, nil
	}
	return s, err
}

// keyExists returns true when the datastore path has the key name.
func (r *render) keyExists(path, name string) (bool, error) {
	l, err := r.keyNames(path)
	if err != nil {
		return false, err
	}
	return slices.Contains(l, name), nil
}

// keyNames returns the names of the keys of the datastore path, matching the
// pattern when one is given, sorted: none when the datastore does not exist.
func (r *render) keyNames(path string, pattern ...string) ([]string, error) {
	p, err := r.dataStore(path)
	if err != nil {
		return nil, err
	}
	r.dataStoreRead("keyNames "+p.String(), p)
	if !r.dataStoreExists(p) {
		return []string{}, nil
	}
	l, err := fetch(r, "keyNames "+p.String(), func() ([]string, error) {
		return r.t.Fetcher.DataKeyNames(r.ctx, p)
	})
	if errors.Is(err, ErrNotFound) {
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}
	glob := optional(pattern, 0)
	names := make([]string, 0, len(l))
	for _, name := range l {
		if glob == "" || fnmatch.Match(glob, name, 0) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// config returns the value of the keyword kw, section.option, of the object
// path, as the node serving the api evaluates it.
func (r *render) config(path, kw string) (string, error) {
	return r.configAs(path, kw, "")
}

// configAs returns the value of the keyword kw of the object path, as node
// evaluates it.
func (r *render) configAs(path, kw, node string) (string, error) {
	p, err := r.path(path)
	if err != nil {
		return "", err
	}
	id := "config " + p.String() + " " + kw + " " + node
	s := p.String()
	r.trackVersion(Read{ID: id, Kinds: configKinds, Selection: s}, func(st *State) string {
		n := node
		if n == "" {
			n = st.servedBy()
		}
		return configVersion(st.Data, s, n)
	})
	return fetch(r, id, func() (string, error) {
		return r.t.Fetcher.ConfigValue(r.ctx, p, kw, node)
	})
}

// resinfo returns the value of the key of the information of the resource rid
// of the object path, on the node, or on the node serving the api when none is
// given.
func (r *render) resinfo(path, rid, key string, node ...string) (string, error) {
	p, err := r.path(path)
	if err != nil {
		return "", err
	}
	n := optional(node, 0)
	if n == "" {
		n = r.st.servedBy()
	}
	s := p.String()
	id := "resinfo " + s + " " + rid + " " + key + " " + n
	r.trackVersion(Read{ID: id, Kinds: resInfoKinds, Selection: s}, func(st *State) string {
		return st.ResourceInfo[ResourceInfoID(s, n)]
	})
	return fetch(r, id, func() (string, error) {
		return r.t.Fetcher.ResourceInfo(r.ctx, p, n, rid, key)
	})
}

// rangeRead returns a read of a range of objects selected by sel: of all of
// them when sel is empty.
func rangeRead(id string, kinds []string, sel string) Read {
	return Read{ID: id, Kinds: kinds, Selection: sel, AllObjects: sel == ""}
}

// objects returns the objects the selector selects, all of them when none is
// given.
func (r *render) objects(sel ...string) ([]Item, error) {
	s := optional(sel, 0)
	return track(r, rangeRead("objects "+s, objectKinds, s), func(st *State) ([]Item, error) {
		return objects(st.Data, s)
	})
}

// objectStatus returns the object path, an error when it does not exist.
func (r *render) objectStatus(path string) (Item, error) {
	p, err := r.path(path)
	if err != nil {
		return nil, err
	}
	s := p.String()
	return track(r, Read{ID: "object " + s, Kinds: objectKinds, Selection: s}, func(st *State) (Item, error) {
		return objectItem(st.Data, s)
	})
}

// objectExists returns true when the object path has a status.
func (r *render) objectExists(path string) (bool, error) {
	p, err := r.path(path)
	if err != nil {
		return false, err
	}
	s := p.String()
	return track(r, Read{ID: "objectExists " + s, Kinds: objectKinds, Selection: s}, func(st *State) (bool, error) {
		_, ok := st.Data.Cluster.Object[s]
		return ok, nil
	})
}

// avail returns the availability status of the object path, an error when it
// does not exist.
func (r *render) avail(path string) (string, error) {
	p, err := r.path(path)
	if err != nil {
		return "", err
	}
	s := p.String()
	return track(r, Read{ID: "avail " + s, Kinds: objectKinds, Selection: s}, func(st *State) (string, error) {
		o, ok := st.Data.Cluster.Object[s]
		if !ok {
			return "", fmt.Errorf("object %s: %w", s, ErrNotFound)
		}
		if o.ActorStatus == nil {
			return "", fmt.Errorf("object %s has no availability status", s)
		}
		return o.Avail.String(), nil
	})
}

// availOr returns the availability status of the object path, def when it does
// not exist.
func (r *render) availOr(path, def string) (string, error) {
	p, err := r.path(path)
	if err != nil {
		return "", err
	}
	s := p.String()
	return track(r, Read{ID: "availOr " + s + " " + def, Kinds: objectKinds, Selection: s}, func(st *State) (string, error) {
		o, ok := st.Data.Cluster.Object[s]
		if !ok || o.ActorStatus == nil {
			return def, nil
		}
		return o.Avail.String(), nil
	})
}

// instances returns the instances of the objects the selector selects, all of
// them when none is given.
func (r *render) instances(sel ...string) ([]Item, error) {
	s := optional(sel, 0)
	return track(r, rangeRead("instances "+s, instanceKinds, s), func(st *State) ([]Item, error) {
		return instances(st.Data, s)
	})
}

// instanceStatus returns the instance of the object path on the node, an error
// when it does not exist.
func (r *render) instanceStatus(path, node string) (Item, error) {
	p, err := r.path(path)
	if err != nil {
		return nil, err
	}
	s := p.String()
	return track(r, Read{ID: "instance " + s + " " + node, Kinds: instanceKinds, Selection: s}, func(st *State) (Item, error) {
		item, err := instanceItem(st.Data, s, node)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, fmt.Errorf("instance %s@%s: %w", s, node, ErrNotFound)
		}
		return item, nil
	})
}

// resources returns the resources of the instances of the objects the selector
// selects, all of them when none is given, of the ids the second argument
// matches, if given.
func (r *render) resources(args ...string) ([]Item, error) {
	s, rid := optional(args, 0), optional(args, 1)
	return track(r, rangeRead("resources "+s+" "+rid, resourceKinds, s), func(st *State) ([]Item, error) {
		return resources(st.Data, s, 0, rid)
	})
}

// groupResources returns the function returning the resources of a driver
// group of the instances of the objects a selector selects.
func (r *render) groupResources(group driver.Group) func(...string) ([]Item, error) {
	return func(sel ...string) ([]Item, error) {
		s := optional(sel, 0)
		return track(r, rangeRead(group.String()+"s "+s, resourceKinds, s), func(st *State) ([]Item, error) {
			return resources(st.Data, s, group, "")
		})
	}
}

// nodes returns the nodes the selector selects: node name patterns and
// label=value, separated by commas. All of them when none is given.
func (r *render) nodes(sel ...string) ([]Item, error) {
	s := optional(sel, 0)
	return track(r, Read{ID: "nodes " + s, Kinds: nodeKinds}, func(st *State) ([]Item, error) {
		return nodes(st.Data, s), nil
	})
}

// nodeLabel returns the value of the label name of the node, empty when the
// node has no such label, an error when the node is not known.
func (r *render) nodeLabel(node, name string) (string, error) {
	return track(r, Read{ID: "nodeLabel " + node + " " + name, Kinds: nodeKinds}, func(st *State) (string, error) {
		n, ok := st.Data.Cluster.Node[node]
		if !ok {
			return "", fmt.Errorf("node %s: %w", node, ErrNotFound)
		}
		return n.Config.Labels[name], nil
	})
}

// networks returns the networks of the nodes, of the name pattern if given.
func (r *render) networks(name ...string) ([]Item, error) {
	s := optional(name, 0)
	return track(r, Read{ID: "networks " + s, Kinds: networkKinds}, func(st *State) ([]Item, error) {
		return networks(st.Data, s)
	})
}
