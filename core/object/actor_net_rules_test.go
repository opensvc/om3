package object_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"

	_ "github.com/opensvc/om3/v3/drivers/resappsimple"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerkvm"
	_ "github.com/opensvc/om3/v3/drivers/restaskhost"
)

// netRuleAlerts returns the alerts the validation of the configuration of a
// svc of the dev namespace has of its net_rules keywords, as
// "<level> <kind> <key>: <comment>".
func netRuleAlerts(t *testing.T, conf string) []string {
	t.Helper()
	p := naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"}
	o, err := object.NewSvc(p, object.WithConfigData([]byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"+conf)), object.WithVolatile(true))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts {
		m := a.Unstructured()
		k, _ := m["key"].(string)
		if k != "app#1.net_rules" && k != "container#1.net_rules" && k != "task#1.net_rules" {
			continue
		}
		l = append(l, m["level"].(string)+" "+m["kind"].(string)+" "+k+": "+m["comment"].(string))
	}
	return l
}

// A rule the grammar refuses, or one no filter is made of yet, is an error
// of the validation, so a configuration holding it is not committed. A rule
// naming a network, a node, an object or a namespace the cluster does not
// have is a warning: it may be written before what it names exists, and
// resolves to nothing until then.
func TestValidateNetRules(t *testing.T) {
	setupKeySources(t)
	caps := []string{
		driver.NewID(driver.GroupApp, "simple").Cap(),
		driver.NewID(driver.GroupContainer, "docker").Cap(),
		driver.NewID(driver.GroupContainer, "kvm").Cap(),
		driver.NewID(driver.GroupTask, "host").Cap(),
	}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	node := hostname.Hostname()

	assert.Empty(t, netRuleAlerts(t,
		"[app#1]\ntype = simple\nstart = /bin/true\n"+
			"net_rules =\n  deny address 10.0.0.5\n  allow localhost\n  allow dns\n  allow cluster\n  allow network default\n  allow address 10.0.0.0/8\n  deny any\n"))

	// The former keywords are keywords no more.
	assert.Empty(t, netRuleAlerts(t, "[app#1]\ntype = simple\nstart = /bin/true\nnet_allow = node "+node+"\nnet_deny = any\n"),
		"nothing is said under the key of net_rules")
	o, err := object.NewSvc(naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"},
		object.WithConfigData([]byte("[DEFAULT]\nnodes = "+node+"\n[app#1]\ntype = simple\nstart = /bin/true\nnet_allow = node "+node+"\n")), object.WithVolatile(true))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	unknown := false
	for _, a := range alerts {
		m := a.Unstructured()
		if k, _ := m["key"].(string); k == "app#1.net_allow" {
			unknown = true
		}
	}
	assert.True(t, unknown, "net_allow is refused as any keyword the driver does not have")

	assert.Equal(t, []string{
		`error unsupported value app#1.net_rules: net_rules line 2: "allow netwrok default": unknown rule, expected one of localhost, dns, any, cluster, address, network, node, object, namespace`,
	}, netRuleAlerts(t,
		"[app#1]\ntype = simple\nstart = /bin/true\n"+
			"net_rules =\n  allow localhost\n  allow netwrok default\n"))
	assert.Equal(t, []string{
		`error unsupported value app#1.net_rules: net_rules line 2: "node n2": expected allow or deny to start a rule, as allow node n2: a rule has one verb and one noun`,
	}, netRuleAlerts(t, "[app#1]\ntype = simple\nstart = /bin/true\nnet_rules =\n  allow localhost\n  node n2\n"))

	// A port is refused by the backend that knows none, and the same
	// rule is one of a kvm container, whose filter is the nftables one.
	assert.Equal(t, []string{
		`error unsupported value app#1.net_rules: net_rules line 1: "deny any port 5432 proto tcp": port 5432: the cgroup filter of the app, task, oci and lxc container resources knows no port, only the nftables filter of container.kvm does`,
	}, netRuleAlerts(t, "[app#1]\ntype = simple\nstart = /bin/true\nnet_rules = deny any port 5432 proto tcp allow any\n"))
	assert.Equal(t, []string{
		`error unsupported value container#1.net_rules: net_rules line 1: "allow node ghost port 22 proto tcp": port 22: the cgroup filter of the app, task, oci and lxc container resources knows no port, only the nftables filter of container.kvm does`,
	}, netRuleAlerts(t, "[container#1]\ntype = docker\nimage = alpine\nnet_rules = allow node ghost port 22 proto tcp\n"))
	assert.Equal(t, []string{
		`warning unresolved network rule container#1.net_rules: allow node ghost port 22 proto tcp: no such node ghost for now, resolves to nothing until it joins the cluster`,
	}, netRuleAlerts(t, "[container#1]\ntype = kvm\nnetwork = default\nnet_rules = deny any port 25 proto tcp allow node ghost port 22 proto tcp\n"))
	assert.Equal(t, []string{
		`error unsupported value container#1.net_rules: net_rules line 1: "deny any port 22": port 22: expected proto tcp or proto udp after the port`,
	}, netRuleAlerts(t, "[container#1]\ntype = kvm\nnetwork = default\nnet_rules = allow node "+node+" deny any port 22\n"))

	assert.Equal(t, []string{
		`warning unresolved network rule app#1.net_rules: deny network default node ghost: no such node ghost for now, resolves to nothing until it joins the cluster`,
		`warning unresolved network rule app#1.net_rules: allow network nowhere: no such network nowhere for now, resolves to nothing until it exists`,
		`warning unresolved network rule app#1.net_rules: allow node ghost: no such node ghost for now, resolves to nothing until it joins the cluster`,
		`warning unresolved network rule container#1.net_rules: allow network nowhere: no such network nowhere for now, resolves to nothing until it exists`,
		`warning unresolved network rule task#1.net_rules: deny node ghost: no such node ghost for now, resolves to nothing until it joins the cluster`,
	}, netRuleAlerts(t,
		"[app#1]\ntype = simple\nstart = /bin/true\n"+
			"net_rules =\n  deny network default node ghost\n  allow network nowhere\n  allow node ghost\n  allow network default\n"+
			"[container#1]\ntype = docker\nimage = alpine\nnet_rules = allow network nowhere\n"+
			"[task#1]\ntype = host\ncommand = /bin/true\nnet_rules = deny node ghost allow any\n"))
}

// A rule that can never match, a rule before it matching all it names, is
// a warning, and so is a list with no allow rule: the configuration is
// committed, and the filter made of the rules as they are written. They
// are judged on the rules alone, on a node knowing nothing of the cluster
// too.
func TestValidateNetRulesOrder(t *testing.T) {
	setupKeySources(t)
	caps := []string{driver.NewID(driver.GroupApp, "simple").Cap(), driver.NewID(driver.GroupContainer, "kvm").Cap()}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	node := hostname.Hostname()
	head := "[app#1]\ntype = simple\nstart = /bin/true\n"

	// The use case of the keyword warns of nothing.
	assert.Empty(t, netRuleAlerts(t, head+"net_rules =\n  allow namespace .\n  allow dns\n  deny cluster\n  deny network default\n  allow any\n"))

	// A rule appended after the last allow any, and a node after the
	// cluster.
	assert.Equal(t, []string{
		`warning ineffective network rules app#1.net_rules: allow node ` + node + `: never matches: deny cluster, written before it, matches all it names`,
		`warning ineffective network rules app#1.net_rules: deny network default: never matches: allow any, written before it, matches all it names`,
	}, netRuleAlerts(t, head+"net_rules =\n  deny cluster\n  allow node "+node+"\n  allow any\n  deny network default\n"))

	// No allow rule.
	assert.Equal(t, []string{
		`warning ineffective network rules app#1.net_rules: no allow rule: nothing is let through but the loopback addresses and the nameservers, which the implicit rules allow, and an allow any written last denies what the rules name alone`,
	}, netRuleAlerts(t, head+"net_rules = deny cluster deny network default\n"))
	assert.Empty(t, netRuleAlerts(t, head+"net_rules = deny cluster deny network default allow any\n"))

	// The ports of a kvm container, and a rule both shadowed and naming
	// what the cluster does not have.
	assert.Equal(t, []string{
		`warning ineffective network rules container#1.net_rules: allow node ghost port 22 proto tcp: never matches: deny any port 1-1024 proto tcp, written before it, matches all it names`,
		`warning unresolved network rule container#1.net_rules: allow node ghost port 22 proto tcp: no such node ghost for now, resolves to nothing until it joins the cluster`,
	}, netRuleAlerts(t, "[container#1]\ntype = kvm\nnetwork = default\nnet_rules = deny any port 1-1024 proto tcp allow node ghost port 22 proto tcp allow cluster\n"))

	// Where nothing is known of the cluster, the rules are still judged
	// on what they say.
	restore := object.SetNetRuleView(func() (netrule.View, error) { return nil, netrule.ErrNoLocalView })
	alerts := netRuleAlerts(t, head+"net_rules = allow any deny node ghost\n")
	restore()
	assert.Equal(t, []string{
		`warning ineffective network rules app#1.net_rules: deny node ghost: never matches: allow any, written before it, matches all it names`,
	}, alerts)
}

// A rule naming an object, a resource or a namespace the cluster does not
// have is a warning naming what the rule resolves to for the namespace of
// the object, its references expanded. With nobody to tell the objects, as
// when the daemon is down, nothing is warned of them, and nothing is ever
// said of what another namespace holds.
func TestValidateNetRulesNamingObjects(t *testing.T) {
	setupKeySources(t)
	caps := []string{driver.NewID(driver.GroupApp, "simple").Cap()}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	conf := "[app#1]\ntype = simple\nstart = /bin/true\n" +
		"net_rules =\n  deny object ./svc/foo\n  allow object ./svc/{name}-db\n  allow object ./svc/db rid ip#9\n  allow object ./svc/db rid ip#1\n  allow object prod/svc/db\n  allow namespace {namespace}2\n  allow object prod/svc/nope\n  allow namespace .\n"

	var asked []string
	restore := object.SetNetRuleObjects(func(objects []naming.Path, namespaces []string) (netrule.Objects, error) {
		for _, p := range objects {
			asked = append(asked, p.String())
		}
		asked = append(asked, namespaces...)
		return netrule.Objects{
			"dev/svc/db":  {"ip#1": nil},
			"prod/svc/db": {},
		}, nil
	})
	alerts := netRuleAlerts(t, conf)
	restore()
	assert.Equal(t, []string{
		`warning unresolved network rule app#1.net_rules: deny object ./svc/foo: no such object dev/svc/foo for now, resolves to nothing until it exists`,
		`warning unresolved network rule app#1.net_rules: allow object ./svc/web-db: no such object dev/svc/web-db for now, resolves to nothing until it exists`,
		`warning unresolved network rule app#1.net_rules: allow object ./svc/db rid ip#9: no such resource ip#9 in the object dev/svc/db for now, resolves to nothing until it exists`,
	}, alerts)
	assert.Subset(t, asked, []string{"dev/svc/web-db", "dev/svc/db", "prod/svc/db", "dev", "dev2", "dev/svc/foo"})

	restore = object.SetNetRuleObjects(func([]naming.Path, []string) (netrule.Objects, error) {
		return nil, errors.New("connection refused")
	})
	alerts = netRuleAlerts(t, conf)
	restore()
	assert.Empty(t, alerts)
}

// A rule reading its addresses from a key of a datastore is warned of when
// the datastore or the key is not there, and of each line of the key that
// is no address, by its text for a cfg and by its number alone for a sec. A
// datastore of another namespace is read as one of the namespace when its
// share keyword shares its keys with it, and is an error when it does not,
// whoever writes the rule.
func TestValidateNetRulesReadingAKey(t *testing.T) {
	setupKeySources(t)
	caps := []string{driver.NewID(driver.GroupApp, "simple").Cap()}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))

	cfg, err := object.NewCfg(naming.Path{Namespace: "dev", Kind: naming.KindCfg, Name: "acl"})
	require.NoError(t, err)
	require.NoError(t, cfg.AddKey("allowed", []byte("10.1.0.0/16\n# the office\n192.168.7.9\n")))
	require.NoError(t, cfg.AddKey("bad", []byte("10.2.0.0/16\nnot an address\n")))
	sec, err := object.NewSec(naming.Path{Namespace: "dev", Kind: naming.KindSec, Name: "acl"})
	require.NoError(t, err)
	require.NoError(t, sec.AddKey("peers", []byte("10.8.2.3\noops\n")))
	other, err := object.NewCfg(naming.Path{Namespace: "prod", Kind: naming.KindCfg, Name: "acl"})
	require.NoError(t, err)
	require.NoError(t, other.AddKey("allowed", []byte("10.9.0.0/16\n")))
	shared, err := object.NewCfg(naming.Path{Namespace: "infra", Kind: naming.KindCfg, Name: "acl"})
	require.NoError(t, err)
	require.NoError(t, shared.AddKey("allowed", []byte("10.10.0.0/16\nnope\n")))
	require.NoError(t, shared.Config().Set(keyop.T{Key: key.Parse("share"), Op: keyop.Set, Value: "qa dev"}))
	require.NoError(t, shared.Config().Commit())
	elsewhere, err := object.NewSec(naming.Path{Namespace: "infra", Kind: naming.KindSec, Name: "qa"})
	require.NoError(t, err)
	require.NoError(t, elsewhere.AddKey("peers", []byte("10.11.2.3\n")))
	require.NoError(t, elsewhere.Config().Set(keyop.T{Key: key.Parse("share"), Op: keyop.Set, Value: "qa"}))
	require.NoError(t, elsewhere.Config().Commit())

	head := "[app#1]\ntype = simple\nstart = /bin/true\n"
	assert.Empty(t, netRuleAlerts(t, head+"net_rules = allow address from dev/cfg/acl key allowed deny cluster\n"))

	assert.Equal(t, []string{
		`warning unresolved network rule app#1.net_rules: allow address from ./cfg/acl key bad: the line 2 of the key bad of the datastore dev/cfg/acl, "not an address", is no address and no range, left out`,
		`warning unresolved network rule app#1.net_rules: allow address from ./sec/acl key peers: the line 2 of the key peers of the datastore dev/sec/acl is no address and no range, left out`,
		`warning unresolved network rule app#1.net_rules: allow address from ./cfg/acl key nope: no such key nope in the datastore dev/cfg/acl for now, resolves to nothing until it is set`,
		`warning unresolved network rule app#1.net_rules: allow address from ./cfg/web key k: no such datastore dev/cfg/web for now, resolves to nothing until it exists`,
	}, netRuleAlerts(t, head+"net_rules =\n  allow address from ./cfg/acl key bad\n  allow address from ./sec/acl key peers\n  allow address from ./cfg/acl key nope\n  allow address from ./cfg/{name} key k\n"))

	assert.Equal(t, []string{
		`error unsupported value app#1.net_rules: deny address from prod/cfg/acl key allowed: the datastore prod/cfg/acl is of another namespace, and its share keyword does not share its keys with the namespace dev, resolves to nothing`,
	}, netRuleAlerts(t, head+"net_rules = deny address from prod/cfg/acl key allowed allow any\n"))

	// A datastore sharing its keys with the namespace of the object is
	// read as one of it: its lines are warned of, by their text, and a
	// key or a datastore that is not there is a warning.
	assert.Equal(t, []string{
		`warning unresolved network rule app#1.net_rules: allow address from infra/cfg/acl key allowed: the line 2 of the key allowed of the datastore infra/cfg/acl, "nope", is no address and no range, left out`,
		`warning unresolved network rule app#1.net_rules: allow address from infra/cfg/acl key later: no such key later in the datastore infra/cfg/acl for now, resolves to nothing until it is set`,
		`warning unresolved network rule app#1.net_rules: allow address from infra/cfg/later key k: no such datastore infra/cfg/later for now, resolves to nothing until it exists`,
	}, netRuleAlerts(t, head+"net_rules =\n  allow address from infra/cfg/acl key allowed\n  allow address from infra/cfg/acl key later\n  allow address from infra/cfg/later key k\n"))

	// One sharing them with other namespaces only is refused, and
	// nothing is said of its keys, there or not.
	assert.Equal(t, []string{
		`error unsupported value app#1.net_rules: allow address from infra/sec/qa key peers: the datastore infra/sec/qa is of another namespace, and its share keyword does not share its keys with the namespace dev, resolves to nothing`,
		`error unsupported value app#1.net_rules: allow address from infra/sec/qa key nope: the datastore infra/sec/qa is of another namespace, and its share keyword does not share its keys with the namespace dev, resolves to nothing`,
	}, netRuleAlerts(t, head+"net_rules =\n  allow address from infra/sec/qa key peers\n  allow address from infra/sec/qa key nope\n"))

	assert.Equal(t, []string{
		`error unsupported value app#1.net_rules: net_rules line 1: "allow address from ./svc/acl key k": ./svc/acl is no datastore, a cfg or a sec, as ./cfg/acl`,
	}, netRuleAlerts(t, head+"net_rules = allow address from ./svc/acl key k\n"))
}
