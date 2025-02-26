package object

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/key"
)

type (
	//
	// Ccfg is the clusterwide configuration store.
	//
	// The content is the same as node.conf, and is overridden by
	// the definition found in node.conf.
	//
	Ccfg struct {
		core
	}
)

var (
	ccfgPrivateKeywords = []keywords.Keyword{
		{
			DefaultText: keywords.NewText(fs, "text/kw/ccfg/id.default"),
			Option:      "id",
			Scopable:    false,
			Section:     "DEFAULT",
			Text:        keywords.NewText(fs, "text/kw/ccfg/id"),
		},
	}

	ccfgKeywordStore = keywords.Store(append(ccfgPrivateKeywords, nodeCommonKeywords...))
)

func NewCluster(opts ...funcopt.O) (*Ccfg, error) {
	return newCcfg(naming.Cluster, opts...)
}

// newCcfg allocates a ccfg kind object.
func newCcfg(path naming.Path, opts ...funcopt.O) (*Ccfg, error) {
	s := &Ccfg{}
	s.path = path
	s.path.Kind = naming.KindCcfg
	err := s.init(s, path, opts...)
	return s, err
}

func (t *Ccfg) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(ccfgKeywordStore, k, t.path.Kind, sectionType)
}

func (t *Ccfg) Name() string {
	k := key.New("cluster", "name")
	return t.config.GetString(k)
}

// Nodes implements Nodes() ([]string, error) to retrieve cluster nodes from config cluster.nodes
// This is required because embedded implementation from core is not valid for ccfg
func (t *Ccfg) Nodes() ([]string, error) {
	k := key.New("cluster", "nodes")
	return t.config.GetStrings(k), nil
}

// DRPNodes implements DRPNodes() ([]string, error) to retrieve cluster drpnodes from config cluster.drpnodes
// This is required because embedded implementation from core is not valid for ccfg
func (t *Ccfg) DRPNodes() ([]string, error) {
	k := key.New("cluster", "drpnodes")
	return t.config.GetStrings(k), nil
}

// GetClusterConfig returns the cached config data if any, or load the cache and return the cached config data.
func GetClusterConfig() (cluster.Config, error) {
	cfg := cluster.ConfigData.Get()
	if cfg != nil {
		return *cfg, nil
	}
	cfg, err := getClusterConfig()
	cluster.ConfigData.Set(cfg)
	return *cfg, err
}

// SetClusterConfig refreshes the config data cache and returns the new config data.
func SetClusterConfig() (cluster.Config, error) {
	cfg, err := getClusterConfig()
	cluster.ConfigData.Set(cfg)
	return *cfg, err
}

// getClusterConfig create the config data from the merged cluster and node configuration files.
func getClusterConfig() (*cluster.Config, error) {
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

		keyNodeSSHKey = key.New("node", "sshkey")
	)

	cfg := &cluster.Config{}
	t, err := NewCluster(WithVolatile(true))
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
	if vip, err := getVip(c, cfg.Nodes); err != nil {
		cfg.Issues = append(cfg.Issues, err.Error())
	} else {
		cfg.Vip = vip
	}
	cfg.Listener.CRL = c.GetString(keyListenerCRL)
	if v, err := c.Eval(keyListenerAddr); err != nil {
		cfg.Issues = append(cfg.Issues, fmt.Sprintf("eval listener addr: %s", err))
	} else {
		cfg.Listener.Addr = v.(string)
	}
	if v, err := c.Eval(keyListenerPort); err != nil {
		cfg.Issues = append(cfg.Issues, fmt.Sprintf("eval listener port: %s", err))
	} else {
		cfg.Listener.Port = v.(int)
	}
	cfg.Listener.OpenIDWellKnown = c.GetString(keyListenerOpenIDWellKnown)
	cfg.Listener.DNSSockGID = c.GetString(keyListenerDNSSockGID)
	cfg.Listener.DNSSockUID = c.GetString(keyListenerDNSSockUID)
	if homedir, err := os.UserHomeDir(); err != nil {
		cfg.Issues = append(cfg.Issues, fmt.Sprintf("user home dir: %s", err))
	} else {
		cfg.SetSSHKeyFile(filepath.Join(homedir, ".ssh", c.GetString(keyNodeSSHKey)))
	}
	return cfg, nil
}

var (
	ErrVIPScope = errors.New("vip scope")
)

// getVip returns the VIP from cluster config
func getVip(c *xconfig.T, nodes []string) (cluster.Vip, error) {
	vip := cluster.Vip{}
	keyVip := key.New("cluster", "vip")
	defaultVip := c.Get(keyVip)
	if defaultVip == "" {
		return vip, nil
	}

	// pickup defaults from vip keyword
	ipname, netmask, dev, err := parseVip(defaultVip)
	if err != nil {
		return cluster.Vip{}, err
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

	vip = cluster.Vip{
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
