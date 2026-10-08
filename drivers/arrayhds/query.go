package arrayhds

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type (
	// element is one node of the xml tree the manager answers a query with.
	//
	// The tree is read whole and its elements are looked up at any depth, as
	// v2 looks them up with ElementTree.iter. The envelope the manager wraps
	// its answer in is then nothing this driver depends on: a lookup through
	// a fixed path of elements reads an envelope it does not know as an array
	// with no volume, no port and no domain, which is the answer a delete or
	// a mapping must never act on.
	element struct {
		name     string
		attrs    map[string]string
		children []*element
	}

	// logicalUnit is one volume of the array.
	//
	// The attributes are kept as the manager names them, so the volume is
	// rendered the way v2 rendered it: the collector reads objectID,
	// displayName, capacityInKB, consumedCapacityInKB, dpPoolID and label
	// from what an array reports, and a form reads the same names from what
	// "add disk" answers.
	logicalUnit struct {
		ObjectID     string
		DevNum       string
		DisplayName  string
		CapacityInKB string
		Label        string
		HasLabel     bool
		Paths        []path
		attrs        map[string]string
	}

	// hostStorageDomain is one domain a volume is exported through.
	hostStorageDomain struct {
		DomainID string
		PortName string
		WWNs     []string
		Paths    []path
	}

	// path is one export of a volume.
	path struct {
		DevNum   string
		DomainID string
		PortName string
		LUN      string
		attrs    map[string]string
	}

	// pool is one pool a volume can be carved from.
	pool struct {
		PoolID string
		Name   string
		attrs  map[string]string
	}

	// port is one port of the array.
	port struct {
		DisplayName string
		WWPN        string
		attrs       map[string]string
	}
)

// decodeXML reads the tree the manager answers a query with.
func decodeXML(out string) (*element, error) {
	dec := xml.NewDecoder(strings.NewReader(out))
	var root *element
	stack := make([]*element, 0)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			e := &element{name: tok.Name.Local, attrs: make(map[string]string, len(tok.Attr))}
			for _, attr := range tok.Attr {
				e.attrs[attr.Name.Local] = attr.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("several root elements")
				}
				root = e
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, e)
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	if root == nil {
		return nil, fmt.Errorf("no xml element")
	}
	return root, nil
}

// iter returns the elements of a name, the element itself included, at any
// depth and in the order they are written, as ElementTree.iter does.
func (t *element) iter(name string) []*element {
	l := make([]*element, 0)
	var walk func(*element)
	walk = func(e *element) {
		if e.name == name {
			l = append(l, e)
		}
		for _, child := range e.children {
			walk(child)
		}
	}
	walk(t)
	return l
}

func copyAttrs(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func newPath(e *element) path {
	return path{
		DevNum:   e.attrs["devNum"],
		DomainID: e.attrs["domainID"],
		PortName: e.attrs["portName"],
		LUN:      e.attrs["LUN"],
		attrs:    copyAttrs(e.attrs),
	}
}

func newLogicalUnit(e *element) logicalUnit {
	t := logicalUnit{
		ObjectID:     e.attrs["objectID"],
		DevNum:       e.attrs["devNum"],
		DisplayName:  e.attrs["displayName"],
		CapacityInKB: e.attrs["capacityInKB"],
		Paths:        make([]path, 0),
		attrs:        copyAttrs(e.attrs),
	}
	for _, p := range e.iter("Path") {
		t.Paths = append(t.Paths, newPath(p))
	}
	for _, ldev := range e.iter("LDEV") {
		for _, label := range ldev.iter("ObjectLabel") {
			if s, ok := label.attrs["label"]; ok {
				t.Label = s
				t.HasLabel = true
			}
		}
	}
	return t
}

func newHostStorageDomain(e *element) hostStorageDomain {
	t := hostStorageDomain{
		DomainID: e.attrs["domainID"],
		PortName: e.attrs["portName"],
		WWNs:     make([]string, 0),
		Paths:    make([]path, 0),
	}
	for _, w := range e.iter("WWN") {
		if s := w.attrs["WWN"]; s != "" {
			t.WWNs = append(t.WWNs, normalizedWWN(s))
		}
	}
	for _, p := range e.iter("Path") {
		t.Paths = append(t.Paths, newPath(p))
	}
	return t
}

func newPool(e *element) pool {
	return pool{
		PoolID: e.attrs["poolID"],
		Name:   e.attrs["name"],
		attrs:  copyAttrs(e.attrs),
	}
}

// newPort reads a port. Its world wide name is worldWidePortName, which is
// what v2 and the collector read. The wwn attribute is read when that one is
// missing, as the earlier versions of this driver read it.
func newPort(e *element) port {
	t := port{
		DisplayName: e.attrs["displayName"],
		attrs:       copyAttrs(e.attrs),
	}
	if s := e.attrs["worldWidePortName"]; s != "" {
		t.WWPN = normalizedWWN(s)
		t.attrs["worldWidePortName"] = t.WWPN
	} else if s := e.attrs["wwn"]; s != "" {
		t.WWPN = normalizedWWN(s)
	}
	return t
}

// MarshalJSON renders a volume as v2 did: its attributes, its paths under
// "Path", and its label under "label" when it has one.
func (t logicalUnit) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(t.attrs)+2)
	for k, v := range t.attrs {
		m[k] = v
	}
	m["Path"] = t.Paths
	if t.HasLabel {
		m["label"] = t.Label
	}
	return json.Marshal(m)
}

// MarshalJSON renders a path as v2 did: its attributes.
func (t path) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.attrs)
}

// MarshalJSON renders a pool as v2 did: its attributes.
func (t pool) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.attrs)
}

// MarshalJSON renders a port as v2 did: its attributes, the world wide name
// normalized.
func (t port) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.attrs)
}

// capacityKB returns the capacity of a volume, in KB.
func (t logicalUnit) capacityKB() (int64, error) {
	i, err := strconv.ParseInt(t.CapacityInKB, 10, 64)
	if err != nil || i < 0 {
		return 0, fmt.Errorf("devnum %s (%s): the array reports a capacity of %q KB, which is not a size", t.DevNum, t.DisplayName, t.CapacityInKB)
	}
	return i, nil
}

// diskID returns the identifier the collector knows a volume by: the last
// two parts of its object id, "<serial>.<n>", as v2 renders it and as the
// collector reads the volumes an array reports.
func (t logicalUnit) diskID() string {
	l := strings.Split(t.ObjectID, ".")
	if len(l) < 2 || l[len(l)-2] == "" || l[len(l)-1] == "" {
		return ""
	}
	return strings.Join(l[len(l)-2:], ".")
}

// normalizedWWN returns a world wide name the way the array stores one, so a
// name written with dots and in upper case matches one written without.
func normalizedWWN(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, ".", ""))
}

// query runs a manager query and reads the xml tree it answers.
func (t *Array) query(ctx context.Context, args ...string) (*element, error) {
	out, err := t.run(ctx, true, true, args...)
	if err != nil {
		return nil, err
	}
	root, err := decodeXML(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: the answer is not an xml tree: %w%s", t.Name(), strings.Join(args, " "), err, outputOf([]byte(out), nil))
	}
	return root, nil
}

// getLogicalUnits returns the volumes of the array, with their exports.
//
// A display name narrows the query to the volumes the manager matches with
// it. What it matches is the manager's business, so a caller looking for one
// volume still picks it by its device number.
func (t *Array) getLogicalUnits(ctx context.Context, displayName string) ([]logicalUnit, error) {
	args := []string{"GetStorageArray", "subtarget=Logicalunit", "lusubinfo=Path,LDEV,VolumeConnection"}
	if displayName != "" {
		args = append(args, "displayname="+displayName)
	}
	root, err := t.query(ctx, args...)
	if err != nil {
		return nil, err
	}
	l := make([]logicalUnit, 0)
	for _, e := range root.iter("LogicalUnit") {
		l = append(l, newLogicalUnit(e))
	}
	return l, nil
}

// getHostStorageDomains returns the domains of the array, with the initiators
// they know and the volumes they export.
func (t *Array) getHostStorageDomains(ctx context.Context) ([]hostStorageDomain, error) {
	root, err := t.query(ctx, "GetStorageArray", "subtarget=HostStorageDomain", "hsdsubinfo=WWN,Path")
	if err != nil {
		return nil, err
	}
	l := make([]hostStorageDomain, 0)
	for _, e := range root.iter("HostStorageDomain") {
		l = append(l, newHostStorageDomain(e))
	}
	return l, nil
}

// getPools returns the pools of the array.
func (t *Array) getPools(ctx context.Context) ([]pool, error) {
	root, err := t.query(ctx, "GetStorageArray", "subtarget=Pool")
	if err != nil {
		return nil, err
	}
	l := make([]pool, 0)
	for _, e := range root.iter("Pool") {
		l = append(l, newPool(e))
	}
	return l, nil
}

// getPorts returns the ports of the array.
func (t *Array) getPorts(ctx context.Context) ([]port, error) {
	root, err := t.query(ctx, "GetStorageArray", "subtarget=Port")
	if err != nil {
		return nil, err
	}
	l := make([]port, 0)
	for _, e := range root.iter("Port") {
		l = append(l, newPort(e))
	}
	return l, nil
}

// unitByDevNum returns the one volume of a device number.
//
// A query narrowed by display name may answer more volumes than the one
// asked for, and the first of them is not the one to resize or to report:
// the volume is picked by its device number, and one missing or listed twice
// is an error.
func unitByDevNum(units []logicalUnit, devNum string) (logicalUnit, error) {
	var (
		found logicalUnit
		n     int
	)
	for _, unit := range units {
		if unit.DevNum == devNum {
			found = unit
			n++
		}
	}
	switch n {
	case 0:
		return found, fmt.Errorf("devnum %s: the array lists no such volume among the %d it answered", devNum, len(units))
	case 1:
		return found, nil
	default:
		return found, fmt.Errorf("devnum %s: the array lists %d volumes of that device number", devNum, n)
	}
}

// poolByName returns the pool a name refers to.
func poolByName(pools []pool, name string) (pool, bool) {
	for _, p := range pools {
		if p.Name == name {
			return p, true
		}
	}
	return pool{}, false
}

// portByTarget returns the port a target is reached through, named by its
// world wide name or by its display name.
func portByTarget(ports []port, target string) (port, bool) {
	target = normalizedWWN(target)
	for _, p := range ports {
		if (p.WWPN != "" && p.WWPN == target) || strings.EqualFold(p.DisplayName, target) {
			return p, true
		}
	}
	return port{}, false
}

// portByName returns the port of a display name.
func portByName(ports []port, name string) (port, bool) {
	for _, p := range ports {
		if p.DisplayName == name {
			return p, true
		}
	}
	return port{}, false
}

// domainsOf returns the domains an initiator reaches a port through.
func domainsOf(domains []hostStorageDomain, hbaID string, portName string) []hostStorageDomain {
	hbaID = normalizedWWN(hbaID)
	l := make([]hostStorageDomain, 0)
	for _, domain := range domains {
		if domain.PortName != portName {
			continue
		}
		for _, w := range domain.WWNs {
			if w == hbaID {
				l = append(l, domain)
				break
			}
		}
	}
	return l
}

// domainOf returns the domain of an id on a port. A domain id is only unique
// on its port: domain 1 of CL1-A is not domain 1 of CL2-A.
func domainOf(domains []hostStorageDomain, portName, domainID string) (hostStorageDomain, bool) {
	for _, domain := range domains {
		if domain.PortName == portName && domain.DomainID == domainID {
			return domain, true
		}
	}
	return hostStorageDomain{}, false
}

// usedLUNs returns the logical unit numbers a domain already hands out.
func usedLUNs(domain hostStorageDomain) []int {
	l := make([]int, 0, len(domain.Paths))
	for _, p := range domain.Paths {
		if i, err := strconv.Atoi(p.LUN); err == nil {
			l = append(l, i)
		}
	}
	return l
}

// freeLUN returns the lowest logical unit number none of the domains hands
// out, so one volume is the same number on every path to it.
func freeLUN(used map[int]bool) int {
	for lun := 0; lun < 65536; lun++ {
		if !used[lun] {
			return lun
		}
	}
	return -1
}
