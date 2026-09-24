package object

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	regexpScalerPrefix = regexp.MustCompile(`^[0-9]+\.`)
)

func (t *core) reloadConfig() error {
	return t.loadConfig(t.config.Referrer)
}

func (t *core) loadConfig(referrer xconfig.Referrer) error {
	var err error
	var sources []any
	if t.configData != nil {
		sources = []any{t.configData}
	} else if t.configFile != "" {
		sources = []any{t.configFile}
	} else {
		sources = []any{}
	}
	if t.config, err = xconfig.NewObject(t.configFile, sources...); err != nil {
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
	if fqdn := t.fqdn(); fqdn != nil {
		return fqdn.String()
	} else {
		return ""
	}
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
		return ref, fmt.Errorf("can't dereference volume head on a non-actor object: %s", ref)
	}
	type header interface {
		Head() string
	}
	if len(l) != 2 {
		return ref, fmt.Errorf("misformatted volume head ref: %s", ref)
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

// regexpHostIDRef matches the references to a host id of a resource:
// {container#1.uid}, {container#1.gid.101}. The first part has to be a
// resource id, so a key like {env.uid} is left to the configuration.
var regexpHostIDRef = regexp.MustCompile(`^([a-z]+#[^.]+)\.(uid|gid)(?:\.([0-9]+))?$`)

// dereferenceHostID answers which host id an id of a resource runs as.
//
// A rootless container runs its root as its user and its other ids as the
// subordinate ids of that user, so a file its uid 101 has to own is owned,
// on the host, by an id only the container can say: the first subordinate id
// of the user plus 100, on this node, whose /etc/subuid may say otherwise
// than another's. A rootful container runs its ids as themselves, so the
// same reference answers the same id, and a configuration naming its owners
// by reference holds whether the container runs rootless or not.
//
// No id is the root of the resource, which is the user of a rootless
// container.
func (t *core) dereferenceHostID(ref string) (string, error) {
	m := regexpHostIDRef.FindStringSubmatch(ref)
	r, err := t.referencedResource(ref, m[2])
	if err != nil {
		return ref, err
	}
	o, ok := r.(resource.IDMapper)
	if !ok {
		return ref, fmt.Errorf("resource referenced by %s cannot say which host ids its ids run as", ref)
	}
	var id uint64
	if m[3] != "" {
		if id, err = strconv.ParseUint(m[3], 10, 32); err != nil {
			return ref, fmt.Errorf("%s: %w", ref, err)
		}
	}
	var hostID uint32
	switch m[2] {
	case "uid":
		hostID, err = o.HostUID(uint32(id))
	default:
		hostID, err = o.HostGID(uint32(id))
	}
	if err != nil {
		return ref, fmt.Errorf("%s: %w", ref, err)
	}
	return strconv.FormatUint(uint64(hostID), 10), nil
}

// dereferenceCapacity answers how big a resource is, in bytes.
//
// It is what a size written as a share of another size is resolved against:
// half of a volume group is half of what the group holds, which only the
// group can say. The capacity is not the size keyword of the resource, which
// says what it was asked to be, but what it is now, so the share follows the
// resource when the resource grows.
func (t *core) dereferenceCapacity(ref string) (string, error) {
	r, err := t.referencedResource(ref, "capacity")
	if err != nil {
		return ref, err
	}
	o, ok := r.(resource.Sizer)
	if !ok {
		return ref, fmt.Errorf("resource referenced by %s cannot say how big it is", ref)
	}
	size, err := o.CurrentSize(context.Background())
	if err != nil {
		return ref, fmt.Errorf("%s: %w", ref, err)
	}
	if size < 0 {
		return ref, fmt.Errorf("%s: has no size yet", ref)
	}
	return strconv.FormatInt(size, 10), nil
}

// dereferenceFree answers how much of a resource nothing has taken yet, in
// bytes.
//
// It is the other half of the capacity: a volume group says how big it is and
// how much of it is unused, and a logical volume carved from it is sized from
// the second. What lvm2 spells "100%FREE" is "$(100% * {disk#vg.free})" here,
// with the difference that om knows the number it lands on, so what the volume
// takes of the pool can be rationed and reported.
func (t *core) dereferenceFree(ref string) (string, error) {
	r, err := t.referencedResource(ref, "free")
	if err != nil {
		return ref, err
	}
	o, ok := r.(resource.Freer)
	if !ok {
		return ref, fmt.Errorf("resource referenced by %s cannot say how much of it is free", ref)
	}
	free, err := o.CurrentFree(context.Background())
	if err != nil {
		return ref, fmt.Errorf("%s: %w", ref, err)
	}
	if free < 0 {
		return ref, fmt.Errorf("%s: has no free space to report", ref)
	}
	return strconv.FormatInt(free, 10), nil
}

// referencedResource is the resource a "<rid>.<what>" reference names.
//
// A resource configured and not built yet postpones the reference rather than
// failing it, which is how a configuration naming one validates before
// anything is provisioned.
func (t *core) referencedResource(ref, what string) (resource.Driver, error) {
	l := strings.SplitN(ref, ".", 2)
	var i any = t.config.Referrer
	actor, ok := i.(Actor)
	if !ok {
		return nil, fmt.Errorf("can't dereference %s on a non-actor object: %s", what, ref)
	}
	if len(l) != 2 {
		return nil, fmt.Errorf("misformatted %s ref: %s", what, ref)
	}
	rid := l[0]
	r := actor.ResourceByID(rid)
	if r == nil {
		if t.config.HasSectionString(rid) {
			return nil, xconfig.NewErrPostponedRef(ref, rid)
		}
		return nil, fmt.Errorf("resource referenced by %s not found", ref)
	}
	return r, nil
}

func (t *core) dereferenceExposedDevices(ref string) (string, error) {
	l := strings.SplitN(ref, ".", 2)
	var i any = t.config.Referrer
	actor, ok := i.(Actor)
	if !ok {
		return ref, fmt.Errorf("can't dereference exposed_devs on a non-actor object: %s", ref)
	}
	type exposedDeviceser interface {
		ExposedDevices(context.Context) device.L
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
	ctx := context.Background()
	xdevs := o.ExposedDevices(ctx)
	ls := make([]string, len(xdevs))
	for i, xd := range xdevs {
		ls[i] = xd.String()
	}
	return strings.Join(ls, " "), nil
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
				continue
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
	case regexpHostIDRef.MatchString(ref):
		return t.dereferenceHostID(ref)
	case strings.HasSuffix(ref, ".capacity"):
		return t.dereferenceCapacity(ref)
	case strings.HasSuffix(ref, ".free"):
		return t.dereferenceFree(ref)
	case strings.HasPrefix(ref, "volume#") && strings.HasSuffix(ref, ".mnt"):
		return t.dereferenceVolumeHead(ref)
	}
	return ref, fmt.Errorf("%w: %s", xconfig.ErrUnknownReference, ref)
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
