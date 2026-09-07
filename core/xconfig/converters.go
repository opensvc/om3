package xconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// TNodesConverter is the type of converter used for the nodes keyword,
	// which makes sure the local nodename is in the resulting []string.
	TNodesConverter struct{}

	// TPeersConverter is the type of converter used for the drpnodes and
	// encapnodes keyword, which accepts to return an empty list.
	TPeersConverter struct{}
)

// NodesConverter and PeersConverter are the singletons the keyword
// definitions must reference. They are hosted here instead of the converters
// package because they depend on the node selector.
var (
	NodesConverter converters.Converter = TNodesConverter{}
	PeersConverter converters.Converter = TPeersConverter{}
)

func init() {
	converters.Register(NodesConverter)
	converters.Register(PeersConverter)
}

func (t TNodesConverter) String() string {
	return "nodes"
}

func (t TNodesConverter) Convert(s string) (interface{}, error) {
	if strings.ContainsRune(s, ',') {
		return nil, fmt.Errorf("invalid node value %q: ',' is not allowed", s)
	}
	l, err := nodeselector.Expand(s)
	if errors.Is(err, nodeselector.ErrClusterNodeCacheEmpty) {
		// pass
	} else if err != nil {
		return nil, err
	}
	if len(l) == 0 && env.Context() == "" {
		return []string{hostname.Hostname()}, nil
	}
	return l, nil
}

func (t TPeersConverter) String() string {
	return "peers"
}

func (t TPeersConverter) Convert(s string) (interface{}, error) {
	return nodeselector.Expand(s)
}
