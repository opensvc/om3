package object

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
)

var (
	regexpScalerPrefix        = regexp.MustCompile(`^[0-9]+\.`)
	regexpExposedDevicesIndex = regexp.MustCompile(`^exposed_devs\[([0-9]+)\]`)
)

func (t *core) reloadConfig() error {
	return t.loadConfig(t.config.Referrer)
}

func (t *core) loadConfig(referrer xconfig.Referrer) error {
	var err error
	var sources []any
	cf := t.ConfigFile()
	if t.configData != nil {
		sources = []any{t.configData}
	} else if cf != "" {
		sources = []any{cf}
	} else {
		sources = []any{}
	}
	if t.config, err = xconfig.NewObject(cf, sources...); err != nil {
		return err
	}
	t.config.Path = t.path
	t.config.Referrer = referrer
	t.config.NodeReferrer, err = t.Node()
	return nil
}

func (t *core) Config() *xconfig.T {
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
	if err := t.config.Set(op); err != nil {
		t.log.Errorf("%s", err)
	}
	return t.id
}

func (t *core) Orchestrate() string {
	k := key.Parse("orchestrate")
	return t.config.GetString(k)
}

func (t *core) FQDN() string {
	return t.fqdn().String()
}

func (t *core) Domain() string {
	return t.fqdn().Domain()
}

func (t *core) fqdn() *naming.FQDN {
	if cluster, err := t.Cluster(); err != nil {
		return nil
	} else {
		return naming.NewFQDN(t.path, cluster.Name())
	}
}

func (t *core) Env() string {
	k := key.Parse("env")
	if s := t.config.GetString(k); s != "" {
		return s
	}
	if node, err := t.Node(); err != nil {
		return "TST"
	} else {
		return node.Env()
	}
}

func (t *core) App() string {
	k := key.Parse("app")
	return t.config.GetString(k)
}

func (t *core) Topology() topology.T {
	k := key.Parse("topology")
	s := t.config.GetString(k)
	return topology.New(s)
}

func (t *core) Placement() placement.Policy {
	k := key.Parse("placement")
	s := t.config.GetString(k)
	return placement.NewPolicy(s)
}

func (t *core) Priority() priority.T {
	k := key.Parse("priority")
	if i, err := t.config.GetIntStrict(k); err != nil {
		//t.log.Error().Err(err).Send()
		return *priority.New()
	} else {
		return priority.T(i)
	}
}

func (t *core) Peers() ([]string, error) {
	impersonate := hostname.Hostname()
	if v, err := t.config.IsInNodes(impersonate); err != nil {
		return nil, err
	} else if v {
		return t.Nodes()
	}
	if v, err := t.config.IsInDRPNodes(impersonate); err != nil {
		return nil, err
	} else if v {
		return t.DRPNodes()
	}
	return nil, fmt.Errorf("node %s has no peers: not in nodes nor drpnodes", impersonate)
}

func (t *core) FlexMin() (int, error) {
	var (
		i, maxValue int
		err         error
	)
	k := key.Parse("flex_min")
	if i, err = t.config.GetIntStrict(k); err != nil {
		return 0, nil
	}
	if i < 0 {
		return 0, nil
	}
	if maxValue, err = t.FlexMax(); err != nil {
		return 0, err
	}
	if i > maxValue {
		return maxValue, nil
	}
	return i, nil
}

func (t *core) FlexMax() (int, error) {
	var (
		i   int
		err error
	)
	nodes, err := t.Peers()
	if err != nil {
		return 0, err
	}
	maxValue := len(nodes)
	k := key.Parse("flex_max")
	if i, err = t.config.GetIntStrict(k); err != nil {
		return maxValue, nil
	}
	if i > maxValue {
		return maxValue, nil
	}
	if i < 0 {
		return 0, nil
	}
	return i, nil
}

func (t *core) FlexTarget() (int, error) {
	var (
		i, minValue, maxValue int
		err                   error
	)
	k := key.Parse("flex_target")
	if i, err = t.config.GetIntStrict(k); err != nil {
		return t.FlexMin()
	}
	if minValue, err = t.FlexMin(); err != nil {
		return 0, err
	}
	if maxValue, err = t.FlexMax(); err != nil {
		return 0, err
	}
	if i < minValue {
		return minValue, nil
	}
	if i > maxValue {
		return maxValue, nil
	}
	return i, nil
}

func (t *core) dereferenceVolumeHead(ref string) (string, error) {
	l := strings.SplitN(ref, ".", 2)
	var i any = t.config.Referrer
	actor, ok := i.(Actor)
	if !ok {
		return ref, fmt.Errorf("can't dereference volume mnt on a non-actor object: %s", ref)
	}
	type header interface {
		Head() string
	}
	if len(l) != 2 {
		return ref, fmt.Errorf("misformatted volume mnt ref: %s", ref)
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
	o, ok := r.(header)
	if !ok {
		return ref, fmt.Errorf("resource referenced by %s has no head mountpoint", ref)
	}
	return o.Head(), nil
}

func (t *core) dereferenceExposedDevices(ref string) (string, error) {
	l := strings.SplitN(ref, ".", 2)
	var i any = t.config.Referrer
	actor, ok := i.(Actor)
	if !ok {
		return ref, fmt.Errorf("can't dereference exposed_devs on a non-actor object: %s", ref)
	}
	type exposedDeviceser interface {
		ExposedDevices() device.L
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
	re := regexp.MustCompile(`exposed_devs\[(?P<Index>[0-9]+)\]`)
	var s string
	matches := re.FindStringSubmatch(l[1])
	if len(matches) == 2 {
		s = matches[1]
	}
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

func (t *core) Dereference(ref string) (string, error) {
	switch ref {
	case "id":
		return t.ID().String(), nil
	case "name", "svcname":
		return t.path.Name, nil
	case "short_name", "short_svcname":
		return strings.SplitN(t.path.Name, ".", 2)[0], nil
	case "scaler_name", "scaler_svcname":
		return regexpScalerPrefix.ReplaceAllString(t.path.Name, ""), nil
	case "scaler_short_name", "scaler_short_svcname":
		return strings.SplitN(regexpScalerPrefix.ReplaceAllString(t.path.Name, ""), ".", 2)[0], nil
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
		return t.Domain(), nil
	case "private_var":
		return t.paths.varDir, nil
	case "initd":
		return filepath.Join(filepath.Dir(t.ConfigFile()), t.path.Name+".d"), nil
	case "collector_api":
		if n, err := t.Node(); err != nil {
			return "", err
		} else if url, err := n.CollectorRestAPIURL(); err != nil {
			return "", err
		} else {
			return url.String(), nil
		}
	case "clusterid":
		if cluster, err := t.Cluster(); err != nil {
			return "", err
		} else {
			return cluster.ID().String(), nil
		}
	case "clustername":
		if cluster, err := t.Cluster(); err != nil {
			return "", err
		} else {
			return cluster.Name(), nil
		}
	case "clusternodes":
		if cluster, err := t.Cluster(); err != nil {
			return "", err
		} else {
			nodes, _ := cluster.Nodes()
			return strings.Join(nodes, " "), nil
		}
	case "clusterdrpnodes":
		return ref, fmt.Errorf("deprecated")
	case "dns":
		if cluster, err := t.Cluster(); err != nil {
			return "", err
		} else {
			l := cluster.Config().GetStrings(key.Parse("cluster.dns"))
			return strings.Join(l, " "), nil
		}
	case "dnsnodes":
		ips := []string{}
		nodes := []string{}
		if cluster, err := t.Cluster(); err != nil {
			return "", err
		} else {
			ips = cluster.Config().GetStrings(key.Parse("cluster.dns"))
			nodes, _ = cluster.Nodes()
		}
		l := make([]string, 0)
		for _, ip := range ips {
			if names, err := net.LookupAddr(ip); err != nil {
				return "", err
			} else {
				for _, name := range names {
					name = strings.TrimSuffix(name, ".")
					if slices.Contains(nodes, name) {
						l = append(l, name)
						break
					}
				}
			}
		}
		if len(l) == 0 {
			return hostname.Hostname(), nil
		}
		return strings.Join(l, " "), nil
	case "dnsuxsock":
		return rawconfig.DNSUDSFile(), nil
	case "dnsuxsockd":
		return rawconfig.DNSUDSDir(), nil
	}
	switch {
	case strings.HasPrefix(ref, "safe://"):
		return ref, fmt.Errorf("todo")
	case strings.Contains(ref, ".exposed_devs"):
		return t.dereferenceExposedDevices(ref)
	case strings.HasPrefix(ref, "volume#") && strings.HasSuffix(ref, ".mnt"):
		return t.dereferenceVolumeHead(ref)
	}
	return ref, fmt.Errorf("unknown reference: %s", ref)
}

func (t *core) Nodes() ([]string, error) {
	l, err := t.config.Eval(key.Parse("nodes"))
	if err != nil {
		return []string{}, err
	}
	return l.([]string), nil
}

func (t *core) DRPNodes() ([]string, error) {
	l, err := t.config.Eval(key.Parse("drpnodes"))
	if err != nil {
		return nil, err
	}
	return l.([]string), nil
}
