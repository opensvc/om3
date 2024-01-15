package object

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/key"
)

type (
	ccfg struct {
		core
	}

	//
	// Ccfg is the clusterwide configuration store.
	//
	// The content is the same as node.conf, and is overriden by
	// the definition found in node.conf.
	//
	Ccfg interface {
		Core
	}
)

var ccfgPrivateKeywords = []keywords.Keyword{
	{
		Section:     "DEFAULT",
		Option:      "id",
		Scopable:    false,
		DefaultText: keywords.NewText(fs, "text/kw/ccfg/id.default"),
		Text:        keywords.NewText(fs, "text/kw/ccfg/id"),
	},
}

var ccfgKeywordStore = keywords.Store(append(ccfgPrivateKeywords, nodeCommonKeywords...))

func NewCluster(opts ...funcopt.O) (*ccfg, error) {
	return newCcfg(naming.Cluster, opts...)
}

// newCcfg allocates a ccfg kind object.
func newCcfg(p any, opts ...funcopt.O) (*ccfg, error) {
	s := &ccfg{}
	err := s.init(s, p, opts...)
	return s, err
}

func (t *ccfg) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(ccfgKeywordStore, k, t.path.Kind, sectionType)
}

func (t *ccfg) Name() string {
	k := key.New("cluster", "name")
	return t.config.GetString(k)
}

// Nodes implements Nodes() ([]string, error) to retrieve cluster nodes from config cluster.nodes
// This is required because embedded implementation from core is not valid for ccfg
func (t *ccfg) Nodes() ([]string, error) {
	k := key.New("cluster", "nodes")
	return t.config.GetStrings(k), nil
}

// DRPNodes implements DRPNodes() ([]string, error) to retrieve cluster drpnodes from config cluster.drpnodes
// This is required because embedded implementation from core is not valid for ccfg
func (t *ccfg) DRPNodes() ([]string, error) {
	k := key.New("cluster", "drpnodes")
	return t.config.GetStrings(k), nil
}
