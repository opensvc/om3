package xconfig

import (
	"errors"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/hostname"
)

type (
	// TNodesConverter is the type of converter used for the nodes keyword,
	// which makes sure the local nodename is in the resulting []string.
	TNodesConverter struct{}

	// TPeersConverter is the type of converter used for the drpnodes and
	// encapnodes keyword, which accepts to return an empty list.
	TPeersConverter struct{}
)

func init() {
	converters.Register(TNodesConverter{})
	converters.Register(TPeersConverter{})
}

func (t TNodesConverter) String() string {
	return "nodes"
}

func (t TNodesConverter) Convert(s string) (interface{}, error) {
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
