package object

import (
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/uri"
)

func init() {
	uri.ClusterSources = clusterSources
}

// clusterSources returns what the cluster allows to fetch from, on top of the
// policy of the node.
func clusterSources() ([]string, []string) {
	node, err := NewNode(WithVolatile(true))
	if err != nil {
		return nil, nil
	}
	cfg := node.MergedConfig()
	return cfg.GetStrings(key.New("node", "allowed_urls")), cfg.GetStrings(key.New("node", "allowed_cidrs"))
}
