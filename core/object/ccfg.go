package object

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/path"
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
	return newCcfg(path.Cluster, opts...)
}

// newCcfg allocates a ccfg kind object.
func newCcfg(p any, opts ...funcopt.O) (*ccfg, error) {
	s := &ccfg{}
	err := s.init(s, p, opts...)
	return s, err
}

func (t ccfg) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(ccfgKeywordStore, k, t.path.Kind, sectionType)
}

func (t ccfg) Name() string {
	k := key.New("cluster", "name")
	return t.config.GetString(k)
}

// ClusterNodes return cluster nodes from config cluster.nodes
func (t ccfg) ClusterNodes() []string {
	k := key.New("cluster", "nodes")
	return t.config.GetStrings(k)
}
