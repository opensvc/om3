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

func (t *core) reloadConfig() error {
	return t.loadConfig(t.config.Referrer)
}

func (t *core) loadConfig(referrer xconfig.Referrer) error {
	var err error
	var sources []interface{}
	if t.configData != nil {
		sources = append(sources, t.configData)
	}
	if t.config, err = xconfig.NewObject(t.ConfigFile(), sources...); err != nil {
		return err
	}
	t.config.Path = t.path
	t.config.Referrer = referrer
	t.config.NodeReferrer = t.Node()
	return err
}

func (t core) Config() *xconfig.T {
	return t.config
}

func (t *core) ID() uuid.UUID {
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

func (t core) Orchestrate() string {
	k := key.Parse("orchestrate")
	return t.config.GetString(k)
}

func (t core) FQDN() string {
	clusterName := rawconfig.ClusterSection().Name
	return fqdn.New(t.path, clusterName).String()
}

func (t core) Env() string {
	k := key.Parse("env")
	if s := t.config.GetString(k); s != "" {
		return s
	}
	return rawconfig.NodeSection().Env
}

func (t core) App() string {
	k := key.Parse("app")
	return t.config.GetString(k)
}

func (t core) Topology() topology.T {
	k := key.Parse("topology")
	s := t.config.GetString(k)
	return topology.New(s)
}

func (t core) Placement() placement.T {
	k := key.Parse("placement")
	s := t.config.GetString(k)
	return placement.New(s)
}

func (t core) Priority() priority.T {
	k := key.Parse("priority")
	if i, err := t.config.GetIntStrict(k); err != nil {
		//t.log.Error().Err(err).Msg("")
		return *priority.New()
	} else {
		return priority.T(i)
	}
}

func (t core) Peers() []string {
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

func (t core) Children() []path.Relation {
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

func (t core) Parents() []path.Relation {
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

func (t core) FlexMin() int {
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

func (t core) FlexMax() int {
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

func (t core) FlexTarget() int {
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

func (t core) dereferenceExposedDevices(ref string) (string, error) {
	l := strings.SplitN(ref, ".", 2)
	var i interface{} = t
	actor, ok := i.(Actor)
	if !ok {
		return ref, fmt.Errorf("can't dereference exposed_devs on a non-actor object: %s", ref)
	}
	type exposedDeviceser interface {
		ExposedDevices() []*device.T
	}
	if len(l) != 2 {
		return ref, fmt.Errorf("misformatted exposed_devs ref: %s", ref)
	}
	rid := l[0]
	r := actor.ResourceByID(rid)
	if r == nil {
		if t.config.HasSectionString(rid) {
			return ref, xconfig.NewErrPostponedRef(ref, rid)
		} else {
			return ref, fmt.Errorf("resource referenced by %s not found", ref)
		}
	}
	o, ok := r.(exposedDeviceser)
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
	idx, err := strconv.Atoi(s)
	if err != nil {
		return ref, fmt.Errorf("misformatted exposed_devs ref: %s", ref)
	}
	xdevs := o.ExposedDevices()
	n := len(xdevs)
	if idx > n-1 {
		return ref, fmt.Errorf("ref %s index error: the referenced resource has only %d exposed devices", ref, n)
	}
	return xdevs[idx].String(), nil
}

func (t core) Dereference(ref string) (string, error) {
	switch ref {
	case "id":
		return t.ID().String(), nil
	case "name", "svcname":
		return t.path.Name, nil
	case "short_name", "short_svcname":
		return strings.SplitN(t.path.Name, ".", 1)[0], nil
	case "scaler_name", "scaler_svcname":
		return RegexpScalerPrefix.ReplaceAllString(t.path.Name, ""), nil
	case "scaler_short_name", "scaler_short_svcname":
		return strings.SplitN(RegexpScalerPrefix.ReplaceAllString(t.path.Name, ""), ".", 1)[0], nil
	case "namespace":
		return t.path.Namespace, nil
	case "kind":
		return t.path.Kind.String(), nil
	case "path", "svcpath":
		if t.path.IsZero() {
			return "", nil
		}
		return t.path.String(), nil
	case "fqdn":
		if t.path.IsZero() {
			return "", nil
		}
		return t.FQDN(), nil
	case "domain":
		if t.path.IsZero() {
			return "", nil
		}
		return fqdn.New(t.path, rawconfig.ClusterSection().Name).Domain(), nil
	case "private_var":
		return t.paths.varDir, nil
	case "initd":
		return filepath.Join(filepath.Dir(t.ConfigFile()), t.path.Name+".d"), nil
	case "collector_api":
		if url, err := t.Node().CollectorRestAPIURL(); err != nil {
			return "", err
		} else {
			return url.String(), nil
		}
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

func (t core) Nodes() []string {
	v := t.config.Get(key.Parse("nodes"))
	l, _ := xconfig.NodesConverter.Convert(v)
	return l.([]string)
}

func (t core) DRPNodes() []string {
	v := t.config.Get(key.Parse("drpnodes"))
	l, _ := xconfig.OtherNodesConverter.Convert(v)
	return l.([]string)
}

func (t core) EncapNodes() []string {
	v := t.config.Get(key.Parse("encapnodes"))
	l, _ := xconfig.OtherNodesConverter.Convert(v)
	return l.([]string)
}

func (t core) HardAffinity() []string {
	l, _ := t.config.Eval(key.Parse("hard_affinity"))
	return l.([]string)
}

func (t core) HardAntiAffinity() []string {
	l, _ := t.config.Eval(key.Parse("hard_anti_affinity"))
	return l.([]string)
}

func (t core) SoftAffinity() []string {
	l, _ := t.config.Eval(key.Parse("soft_affinity"))
	return l.([]string)
}

func (t core) SoftAntiAffinity() []string {
	l, _ := t.config.Eval(key.Parse("soft_anti_affinity"))
	return l.([]string)
}
