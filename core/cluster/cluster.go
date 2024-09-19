package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/util/key"
)

type (
	// Data describes the full Cluster state.
	Data struct {
		Cluster Cluster `json:"cluster"`

		Daemon daemonsubsystem.DaemonLocal `json:"daemon"`
	}

	Cluster struct {
		Config Config                   `json:"config"`
		Status Status                   `json:"status"`
		Object map[string]object.Status `json:"object"`

		Node map[string]node.Node `json:"node"`
	}

	Status struct {
		IsCompat bool `json:"is_compat"`
		IsFrozen bool `json:"is_frozen"`
	}
)

func (s *Data) DeepCopy() *Data {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	newStatus := Data{}
	if err := json.Unmarshal(b, &newStatus); err != nil {
		panic(err)
	}
	return &newStatus
}

func (s *Data) ObjectPaths() naming.Paths {
	allPaths := make(naming.Paths, len(s.Cluster.Object))
	i := 0
	for p := range s.Cluster.Object {
		path, _ := naming.ParsePath(p)
		allPaths[i] = path
		i++
	}
	return allPaths
}

// WithSelector purges the dataset from objects not matching the selector expression
func (s *Data) WithSelector(selector string) *Data {
	if selector == "" {
		return s
	}
	paths, err := objectselector.New(
		selector,
		objectselector.WithPaths(s.ObjectPaths()),
	).Expand()
	if err != nil {
		return s
	}
	selected := paths.StrMap()
	for nodename, nodeData := range s.Cluster.Node {
		for ps := range nodeData.Instance {
			if !selected.Has(ps) {
				delete(s.Cluster.Node[nodename].Instance, ps)
			}
		}
	}
	for ps := range s.Cluster.Object {
		if !selected.Has(ps) {
			delete(s.Cluster.Object, ps)
		}
	}
	return s
}

// WithNamespace purges the dataset from objects not matching the namespace
func (s *Data) WithNamespace(namespace string) *Data {
	if namespace == "" {
		return s
	}
	for nodename, nodeData := range s.Cluster.Node {
		for ps := range nodeData.Instance {
			p, _ := naming.ParsePath(ps)
			if p.Namespace != namespace {
				delete(s.Cluster.Node[nodename].Instance, ps)
			}
		}
	}
	for ps := range s.Cluster.Object {
		p, _ := naming.ParsePath(ps)
		if p.Namespace != namespace {
			delete(s.Cluster.Object, ps)
		}
	}
	return s
}

// GetNodeData extracts from the cluster dataset all information relative
// to node data.
func (s *Data) GetNodeData(nodename string) *node.Node {
	if nodeData, ok := s.Cluster.Node[nodename]; ok {
		return &nodeData
	}
	return nil
}

// GetNodeStatus extracts from the cluster dataset all information relative
// to node status.
func (s *Data) GetNodeStatus(nodename string) *node.Status {
	if nodeData, ok := s.Cluster.Node[nodename]; ok {
		return &nodeData.Status
	}
	return nil
}

// GetObjectStatus extracts from the cluster dataset all information relative
// to an object.
func (s *Data) GetObjectStatus(p naming.Path) object.Digest {
	ps := p.String()
	data := object.NewStatus()
	data.Path = p
	data.IsCompat = s.Cluster.Status.IsCompat
	data.Object, _ = s.Cluster.Object[ps]
	for nodename, ndata := range s.Cluster.Node {
		instanceStates := instance.States{}
		instanceStates.Path = p
		instanceStates.Node.FrozenAt = ndata.Status.FrozenAt
		instanceStates.Node.Name = nodename
		inst, ok := ndata.Instance[ps]
		if !ok {
			continue
		}
		if inst.Status != nil {
			instanceStates.Status = *inst.Status
		}
		if inst.Config != nil {
			instanceStates.Config = *inst.Config
		}
		if inst.Monitor != nil {
			instanceStates.Monitor = *inst.Monitor
		}
		data.Instances = append(data.Instances, instanceStates)
	}
	return *data
}

func GetConfig() (Config, error) {
	var (
		keyID         = key.New("cluster", "id")
		keySecret     = key.New("cluster", "secret")
		keyName       = key.New("cluster", "name")
		keyNodes      = key.New("cluster", "nodes")
		keyDNS        = key.New("cluster", "dns")
		keyCASecPaths = key.New("cluster", "ca")
		keyQuorum     = key.New("cluster", "quorum")

		keyListenerCRL             = key.New("listener", "crl")
		keyListenerAddr            = key.New("listener", "addr")
		keyListenerPort            = key.New("listener", "port")
		keyListenerOpenIDWellKnown = key.New("listener", "openid_well_known")
		keyListenerDNSSockUID      = key.New("listener", "dns_sock_uid")
		keyListenerDNSSockGID      = key.New("listener", "dns_sock_gid")
	)

	cfg := Config{}
	t, err := object.NewCluster(object.WithVolatile(true))
	if err != nil {
		return cfg, err
	}
	c := t.Config()
	cfg.ID = c.GetString(keyID)
	cfg.DNS = c.GetStrings(keyDNS)
	cfg.Nodes = c.GetStrings(keyNodes)
	cfg.Name = c.GetString(keyName)
	cfg.CASecPaths = c.GetStrings(keyCASecPaths)
	cfg.SetSecret(c.GetString(keySecret))
	cfg.Quorum = c.GetBool(keyQuorum)
	var errs error
	if vip, err := getVip(c, cfg.Nodes); err != nil {
		errs = errors.Join(errs, err)
	} else {
		cfg.Vip = vip
	}
	cfg.Listener.CRL = c.GetString(keyListenerCRL)
	if v, err := c.Eval(keyListenerAddr); err != nil {
		errs = errors.Join(errs, fmt.Errorf("eval listener addr: %s", err))
	} else {
		cfg.Listener.Addr = v.(string)
	}
	if v, err := c.Eval(keyListenerPort); err != nil {
		errs = errors.Join(errs, fmt.Errorf("eval listener port: %s", err))
	} else {
		cfg.Listener.Port = v.(int)
	}
	cfg.Listener.OpenIDWellKnown = c.GetString(keyListenerOpenIDWellKnown)
	cfg.Listener.DNSSockGID = c.GetString(keyListenerDNSSockGID)
	cfg.Listener.DNSSockUID = c.GetString(keyListenerDNSSockUID)
	return cfg, errs
}

// VIP returns the VIP from cluster config
var (
	ErrVIPScope = errors.New("vip scope")
)

func getVip(c *xconfig.T, nodes []string) (Vip, error) {
	vip := Vip{}
	keyVip := key.New("cluster", "vip")
	defaultVip := c.Get(keyVip)
	if defaultVip == "" {
		return vip, nil
	}

	// pickup defaults from vip keyword
	ipname, netmask, dev, err := parseVip(defaultVip)
	if err != nil {
		return Vip{}, err
	}

	devs := make(map[string]string)

	var errs error

	for _, n := range nodes {
		v, err := c.EvalAs(keyVip, n)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%w: %s: %w", ErrVIPScope, n, err))
			continue
		}
		customVip := v.(string)
		if customVip == "" || customVip == defaultVip {
			continue
		}
		if _, _, customDev, err := parseVip(customVip); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%w: %s: %w", ErrVIPScope, n, err))
			continue
		} else if customDev != dev {
			devs[n] = customDev
		}
	}

	vip = Vip{
		Default: defaultVip,
		Addr:    ipname,
		Netmask: netmask,
		Dev:     dev,
		Devs:    devs,
	}

	return vip, errs
}

func parseVip(s string) (ipname, netmask, ipdev string, err error) {
	r := strings.Split(s, "@")
	if len(r) != 2 {
		err = fmt.Errorf("unexpected vip value: missing @ in %s", s)
		return
	}
	if len(r[1]) == 0 {
		err = fmt.Errorf("unexpected vip value: empty addr in %s", s)
		return
	}
	ipdev = r[1]
	r = strings.Split(r[0], "/")
	if len(r) != 2 {
		err = fmt.Errorf("unexpected vip value: missing / in %s", s)
		return
	}
	if len(r[0]) == 0 {
		err = fmt.Errorf("unexpected vip value: empty ipname in %s", s)
		return
	}
	ipname = r[0]
	if len(r[1]) == 0 {
		err = fmt.Errorf("unexpected vip value: empty netmask in %s", s)
		return
	}
	netmask = r[1]
	return
}
