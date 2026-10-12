package object

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

func (t *Node) RawConfig() (rawconfig.T, error) {
	return t.config.Raw(), nil
}

func (t *Node) EvalConfig() (rawconfig.T, error) {
	return t.config.RawEvaluatedAs("")
}

func (t *Node) EvalConfigAs(impersonate string) (rawconfig.T, error) {
	return t.config.RawEvaluatedAs(impersonate)
}

func (t *Node) Log() *plog.Logger {
	return t.log
}

func (t *Node) Exists() bool {
	return true
}

func (t *Node) ConfigFile() string {
	return t.configFile
}

func (t *Node) ClusterConfigFile() string {
	return t.clusterConfigFile
}

func (t *Node) loadConfig() error {
	var sources []any

	if t.configData != nil {
		sources = []any{t.configData}
	} else if t.configFile != "" {
		sources = []any{t.configFile}
	} else {
		sources = []any{}
	}

	if config, err := xconfig.NewObject(t.configFile, sources...); err != nil {
		return err
	} else {
		t.config = config
		t.config.Referrer = t
	}
	if t.clusterConfigFile != "" {
		sources = append([]any{t.clusterConfigFile}, sources...)
	}
	if config, err := xconfig.NewObject(t.clusterConfigFile, sources...); err != nil {
		return err
	} else {
		t.mergedConfig = config
		t.mergedConfig.Referrer = t
	}
	return nil
}

func (t *Node) Config() *xconfig.T {
	return t.config
}

func (t *Node) MergedConfig() *xconfig.T {
	return t.mergedConfig
}

func (t *Node) ID() uuid.UUID {
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
	if err := t.config.Set(op); err != nil {
		t.log.Errorf("%s", err)
	}
	return t.id
}

// Env returns the env of the node, the node.env keyword of the node or of
// the cluster configuration.
func (t *Node) Env() string {
	if s := t.mergedConfig.GetString(key.New("node", "env")); s != "" {
		return s
	}
	return "TST"
}

// AllowEnvs returns the envs of the objects allowed to run on this node, nil
// when the node env decides: the node.allow_envs keyword of the node or of
// the cluster configuration.
func (t *Node) AllowEnvs() []string {
	return t.mergedConfig.GetStrings(key.New("node", "allow_envs"))
}

// AllowsEnv reports whether an object of the env may run on this node.
func (t *Node) AllowsEnv(env string) bool {
	return allowsEnv(env, t.Env(), t.AllowEnvs())
}

// allowsEnv reports whether an object of the env may run on a node of the
// nodeEnv env, allowing the allowEnvs envs.
//
// A node allowing no env names allows every env but on a PRD node, which
// allows the PRD objects only: a PRD object may run on any node, as in a DRP
// situation, while the others may not touch the production. A node allowing
// envs allows those alone, so a PRD cluster can host the objects of another
// env, as PPRD, its operators trust it with.
func allowsEnv(env, nodeEnv string, allowEnvs []string) bool {
	if len(allowEnvs) > 0 {
		return slices.Contains(allowEnvs, env)
	}
	return env == "PRD" || nodeEnv != "PRD"
}

func (t *Node) App() string {
	k := key.Parse("app")
	return t.config.GetString(k)
}

func (t *Node) Dereference(ref string) (string, error) {
	switch ref {
	case "id":
		return t.ID().String(), nil
	case "name", "nodename":
		return hostname.Hostname(), nil
	case "short_name", "short_nodename":
		return strings.SplitN(hostname.Hostname(), ".", 2)[0], nil
	case "dnsuxsock":
		return rawconfig.DNSUDSFile(), nil
	case "dnsuxsockd":
		return rawconfig.DNSUDSDir(), nil
	}
	switch {
	case strings.HasPrefix(ref, "safe://"):
		return ref, fmt.Errorf("todo")
	}
	return ref, fmt.Errorf("unknown reference: %s", ref)
}

func (t *Node) PostCommit() error {
	return nil
}

func (t *Node) Nodes() ([]string, error) {
	k := key.T{Section: "cluster", Option: "nodes"}
	nodes := t.MergedConfig().GetStrings(k)
	if len(nodes) == 0 {
		return []string{hostname.Hostname()}, nil
	}
	return nodes, nil
}

func (t *Node) DRPNodes() ([]string, error) {
	return []string{}, nil
}

func (t *Node) EncapNodes() ([]string, error) {
	return []string{}, nil
}

func (t *Node) Nameservers() ([]string, error) {
	dns, err := t.MergedConfig().Eval(key.T{Section: "cluster", Option: "dns"})
	return dns.([]string), err
}

func (t *Node) CNIConfig() (string, error) {
	if s, err := t.MergedConfig().Eval(key.T{Section: "cni", Option: "config"}); err != nil {
		return "", err
	} else {
		return s.(string), nil
	}
}

func (t *Node) CNIPlugins() (string, error) {
	if s, err := t.MergedConfig().Eval(key.T{Section: "cni", Option: "plugins"}); err != nil {
		return "", err
	} else {
		return s.(string), nil
	}
}
