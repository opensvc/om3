package object

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/key"
)

type (
	//
	// Ccfg is the clusterwide configuration store.
	//
	// The content is the same as node.conf, and is overriden by
	// the definition found in node.conf.
	//
	Ccfg struct {
		core
	}
)

var ccfgPrivateKeywords = []keywords.Keyword{
	{
		DefaultText: keywords.NewText(fs, "text/kw/ccfg/id.default"),
		Option:      "id",
		Scopable:    false,
		Section:     "DEFAULT",
		Text:        keywords.NewText(fs, "text/kw/ccfg/id"),
	},
}

var ccfgKeywordStore = keywords.Store(append(ccfgPrivateKeywords, nodeCommonKeywords...))

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
