package xconfig

import (
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
)

type (
	// TNodesConverter is the type of converter used for the nodes keyword,
	// which makes sure the local nodename is in the resulting []string.
	TNodesConverter string

	// TOtherNodesConverter is the type of converter used for the drpnodes and
	// encapnodes keyword, which accepts to return an empty list.
	TOtherNodesConverter string
)

var (
	NodesConverter      TNodesConverter
	OtherNodesConverter TOtherNodesConverter
)

func (t TNodesConverter) String() string {
	return "nodes"
}

func (t TNodesConverter) Convert(s string) (interface{}, error) {
	l, err := nodeselector.LocalExpand(s)
	if err != nil {
		return nil, err
	}
	if len(l) == 0 && env.Context() == "" {
		return []string{hostname.Hostname()}, nil
	}
	return l, nil
}

func (t TOtherNodesConverter) String() string {
	return "other-nodes"
}

func (t TOtherNodesConverter) Convert(s string) (interface{}, error) {
	return nodeselector.LocalExpand(s)
}
