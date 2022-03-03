package object

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"opensvc.com/opensvc/core/fqdn"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/priority"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

var (
	RegexpScalerPrefix        = regexp.MustCompile(`^[0-9]+\.`)
	regexpExposedDevicesIndex = regexp.MustCompile(`.*\.exposed_devs\[([0-9]+)\]`)
)

func (t *Base) reloadConfig() error {
	return t.loadConfig(t.config.Referrer)
}

func (t *Base) loadConfig(referrer xconfig.Referrer) error {
	var err error
	if t.config, err = xconfig.NewObject(t.ConfigFile()); err != nil {
		return err
	}
	t.config.Path = t.Path
	t.config.Referrer = referrer
	t.config.NodeReferrer = t.Node()
	return err
}

func (t Base) Config() *xconfig.T {
	return t.config
}

func (t *Base) ID() uuid.UUID {
	if t.id != uuid.Nil {
		return t.id
	}
	idKey := key.Parse("id")
	if t.config.HasKey(idKey) {
		idStr := t.config.Get(idKey)
		if id, err := uuid.Parse(idStr); err == nil {
			t.id = id
			return t.id
		}
	}
	t.id = uuid.New()
	op := keyop.T{
		Key:   key.Parse("id"),
		Op:    keyop.Set,
		Value: t.id.String(),
	}
	_ = t.config.Set(op)
	if err := t.config.Commit(); err != nil {
		t.log.Error().Err(err).Msg("")
	}
	return t.id
}

func (t Base) Orchestrate() string {
	k := key.Parse("orchestrate")
	return t.config.GetString(k)
}

func (t Base) FQDN() string {
	return fqdn.New(t.Path, rawconfig.Node.Cluster.Name).String()
}

func (t Base) Env() string {
	k := key.Parse("env")
	if s := t.config.GetString(k); s != "" {
		return s
	}
	return rawconfig.Node.Node.Env
}

func (t Base) App() string {
	k := key.Parse("app")
	return t.config.GetString(k)
}

func (t Base) Topology() topology.T {
	k := key.Parse("topology")
	s := t.config.GetString(k)
	return topology.New(s)
}

func (t Base) Placement() placement.T {
	k := key.Parse("placement")
	s := t.config.GetString(k)
	return placement.New(s)
}

func (t Base) Priority() priority.T {
	k := key.Parse("priority")
	if i, err := t.config.GetIntStrict(k); err != nil {
		//t.log.Error().Err(err).Msg("")
		return *priority.New()
	} else {
		return priority.T(i)
	}
}

func (t Base) Peers() []string {
	impersonate := hostname.Hostname()
	switch {
	case t.config.IsInNodes(impersonate):
		return t.Nodes()
	case t.config.IsInDRPNodes(impersonate):
		return t.DRPNodes()
	default:
		return []string{}
	}
}

func (t Base) Children() []path.Relation {
	data := make([]path.Relation, 0)
	k := key.Parse("children")
	l, err := t.config.GetSliceStrict(k)
	if err != nil {
		t.log.Error().Err(err).Msg("")
		return data
	}
	for _, e := range l {
		data = append(data, path.Relation(e))
	}
	return data
}

func (t Base) Parents() []path.Relation {
	data := make([]path.Relation, 0)
	k := key.Parse("parents")
	l, err := t.config.GetSliceStrict(k)
	if err != nil {
		t.log.Error().Err(err).Msg("")
		return data
	}
	for _, e := range l {
		data = append(data, path.Relation(e))
	}
	return data
}

func (t Base) FlexMin() int {
	var (
		i   int
		err error
	)
	k := key.Parse("flex_min")
	if i, err = t.config.GetIntStrict(k); err != nil {
		//t.log.Error().Err(err).Msg("")
		return 0
	}
	if i < 0 {
		return 0
	}
	max := t.FlexMax()
	if i > max {
		return max
	}
	return i
}

func (t Base) FlexMax() int {
	var (
		i   int
		err error
	)
	max := len(t.Peers())
	k := key.Parse("flex_max")
	if i, err = t.config.GetIntStrict(k); err != nil {
		//t.log.Error().Err(err).Msg("")
		return max
	}
	if i > max {
		return max
	}
	if i < 0 {
		return 0
	}
	return i
}

func (t Base) FlexTarget() int {
	var (
		i   int
		err error
	)
	k := key.Parse("flex_target")
	if i, err = t.config.GetIntStrict(k); err != nil {
		//t.log.Error().Err(err).Msg("")
		return t.FlexMin()
	}
	min := t.FlexMin()
	max := t.FlexMax()
	if i < min {
		return min
	}
	if i > max {
		return max
	}
	return i
}

func (t Base) dereferenceExposedDevices(ref string) (string, error) {
	l := strings.SplitN(ref, ".", 2)
	type ExposedDeviceser interface {
		ExposedDevices() []*device.T
	}
	if len(l) != 2 {
		return ref, fmt.Errorf("misformatted exposed_devs ref: %s", ref)
	}
	rid := l[0]
	r := t.ResourceByID(rid)
	if r == nil {
		if t.config.HasSectionString(rid) {
			return ref, xconfig.NewErrPostponedRef(ref, rid)
		} else {
			return ref, fmt.Errorf("resource referenced by %s not found", ref)
		}
	}
	o, ok := r.(ExposedDeviceser)
	if !ok {
		return ref, fmt.Errorf("resource referenced by %s has no exposed devices", ref)
	}
	s := regexpExposedDevicesIndex.FindString(l[1])
	if s == "" {
		xdevs := o.ExposedDevices()
		ls := make([]string, len(xdevs))
		for i, xd := range xdevs {
			ls[i] = xd.String()
		}
		return strings.Join(ls, " "), nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return ref, fmt.Errorf("misformatted exposed_devs ref: %s", ref)
	}
	xdevs := o.ExposedDevices()
	n := len(xdevs)
	if i > n-1 {
		return ref, fmt.Errorf("ref %s index error: the referenced resource has only %d exposed devices", ref, n)
	}
	return xdevs[i].String(), nil
}

func (t Base) Dereference(ref string) (string, error) {
	switch ref {
	case "id":
		return t.ID().String(), nil
	case "name", "svcname":
		return t.Path.Name, nil
	case "short_name", "short_svcname":
		return strings.SplitN(t.Path.Name, ".", 1)[0], nil
	case "scaler_name", "scaler_svcname":
		return RegexpScalerPrefix.ReplaceAllString(t.Path.Name, ""), nil
	case "scaler_short_name", "scaler_short_svcname":
		return strings.SplitN(RegexpScalerPrefix.ReplaceAllString(t.Path.Name, ""), ".", 1)[0], nil
	case "namespace":
		return t.Path.Namespace, nil
	case "kind":
		return t.Path.Kind.String(), nil
	case "path", "svcpath":
		if t.Path.IsZero() {
			return "", nil
		}
		return t.Path.String(), nil
	case "fqdn":
		if t.Path.IsZero() {
			return "", nil
		}
		return t.FQDN(), nil
	case "domain":
		if t.Path.IsZero() {
			return "", nil
		}
		return fqdn.New(t.Path, rawconfig.Node.Cluster.Name).Domain(), nil
	case "private_var":
		return t.paths.varDir, nil
	case "initd":
		return filepath.Join(filepath.Dir(t.ConfigFile()), t.Path.Name+".d"), nil
	case "collector_api":
		return ref, fmt.Errorf("TODO")
	case "clusterid":
		return ref, fmt.Errorf("TODO")
	case "clustername":
		return ref, fmt.Errorf("TODO")
	case "clusternodes":
		return ref, fmt.Errorf("TODO")
	case "clusterdrpnodes":
		return ref, fmt.Errorf("TODO")
	case "dns":
		return ref, fmt.Errorf("TODO")
	case "dnsnodes":
		return ref, fmt.Errorf("TODO")
	case "dnsuxsock":
		return t.Node().DNSUDSFile(), nil
	case "dnsuxsockd":
		return t.Node().DNSUDSDir(), nil
	}
	switch {
	case strings.HasPrefix(ref, "safe://"):
		return ref, fmt.Errorf("TODO")
	case strings.Contains(ref, ".exposed_devs"):
		return t.dereferenceExposedDevices(ref)
	}
	return ref, fmt.Errorf("unknown reference: %s", ref)
}

func (t Base) Nodes() []string {
	v := t.config.Get(key.Parse("nodes"))
	l, _ := xconfig.NodesConverter.Convert(v)
	return l.([]string)
}

func (t Base) DRPNodes() []string {
	v := t.config.Get(key.Parse("drpnodes"))
	l, _ := xconfig.OtherNodesConverter.Convert(v)
	return l.([]string)
}

func (t Base) EncapNodes() []string {
	v := t.config.Get(key.Parse("encapnodes"))
	l, _ := xconfig.OtherNodesConverter.Convert(v)
	return l.([]string)
}

func (t Base) HardAffinity() []string {
	l, _ := t.config.Eval(key.Parse("hard_affinity"))
	return l.([]string)
}

func (t Base) HardAntiAffinity() []string {
	l, _ := t.config.Eval(key.Parse("hard_anti_affinity"))
	return l.([]string)
}

func (t Base) SoftAffinity() []string {
	l, _ := t.config.Eval(key.Parse("soft_affinity"))
	return l.([]string)
}

func (t Base) SoftAntiAffinity() []string {
	l, _ := t.config.Eval(key.Parse("soft_anti_affinity"))
	return l.([]string)
}
