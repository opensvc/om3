package object

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/fqdn"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/priority"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

var (
	RegexpScalerPrefix = regexp.MustCompile(`^[0-9]+\.`)
)

func (t *Base) loadConfig() error {
	var err error
	if t.config, err = xconfig.NewObject(t.ConfigFile()); err != nil {
		return err
	}
	t.config.Path = t.Path
	t.config.Referrer = t
	return err
}

func (t Base) Config() *xconfig.T {
	return t.config
}

func (t Base) ID() uuid.UUID {
	if t.id != uuid.Nil {
		return t.id
	}
	idKey := key.Parse("id")
	if idStr := t.config.GetString(idKey); idStr != "" {
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

func (t Base) Env() string {
	k := key.Parse("env")
	if s := t.config.GetString(k); s != "" {
		return s
	}
	return config.Node.Node.Env
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
		t.log.Error().Err(err).Msg("")
		return *priority.New()
	} else {
		return priority.T(i)
	}
}

func (t Base) Peers() []string {
	impersonate := hostname.Hostname()
	switch {
	case t.config.IsInNodes(impersonate):
		return t.config.Nodes()
	case t.config.IsInDRPNodes(impersonate):
		return t.config.DRPNodes()
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
		t.log.Error().Err(err).Msg("")
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
	k := key.Parse("flex_max")
	if i, err = t.config.GetIntStrict(k); err != nil {
		t.log.Error().Err(err).Msg("")
		return len(t.Peers())
	}
	max := len(t.Peers())
	if i > max {
		return max
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
		t.log.Error().Err(err).Msg("")
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

func (t Base) Dereference(ref string) string {
	switch ref {
	case "id":
		return t.ID().String()
	case "name", "{svcname}":
		return t.Path.Name
	case "short_name", "{short_svcname}":
		return strings.SplitN(t.Path.Name, ".", 1)[0]
	case "scaler_name", "{scaler_svcname}":
		return RegexpScalerPrefix.ReplaceAllString(t.Path.Name, "")
	case "scaler_short_name", "{scaler_short_svcname}":
		return strings.SplitN(RegexpScalerPrefix.ReplaceAllString(t.Path.Name, ""), ".", 1)[0]
	case "namespace":
		return t.Path.Namespace
	case "kind":
		return t.Path.Kind.String()
	case "path", "{svcpath}":
		if t.Path.IsZero() {
			return ""
		}
		return t.Path.String()
	case "fqdn":
		if t.Path.IsZero() {
			return ""
		}
		return fqdn.New(t.Path, config.Node.Cluster.Name).String()
	case "domain":
		if t.Path.IsZero() {
			return ""
		}
		return fqdn.New(t.Path, config.Node.Cluster.Name).Domain()
	case "private_var":
		return t.paths.varDir
	case "initd":
		return filepath.Join(filepath.Dir(t.ConfigFile()), t.Path.Name+".d")
	case "collector_api":
		return "TODO"
	case "clusterid":
		return "TODO"
	case "clustername":
		return "TODO"
	case "clusternodes":
		return "TODO"
	case "clusterdrpnodes":
		return "TODO"
	case "dns":
		return "TODO"
	case "dnsnodes":
		return "TODO"
	case "dnsuxsock":
		return t.Node().DNSUDSFile()
	case "dnsuxsockd":
		return t.Node().DNSUDSDir()
	}
	switch {
	case strings.HasPrefix(ref, "safe://"):
		return "TODO"
	}
	return ref
}

func (t Base) PostCommit() error {
	return nil
}
