package object

import (
	"github.com/pkg/errors"
	"opensvc.com/opensvc/config"
)

var (
	ErrInvalidNode = errors.New("invalid node")
)

func (t *Base) validateAction() error {
	if t.Env() != "PRD" && config.Node.Node.Env == "PRD" {
		return errors.Wrapf(ErrInvalidNode, "not allowed to run on this node (svc env=%s node env=%s)", t.Env(), config.Node.Node.Env)
	}
	if t.config.IsInNodes(config.Node.Hostname) {
		return nil
	}
	if t.config.IsInDRPNodes(config.Node.Hostname) {
		return nil
	}
	return errors.Wrapf(ErrInvalidNode, "hostname '%s' is not a member of DEFAULT.nodes, DEFAULT.drpnode nor DEFAULT.drpnodes", config.Node.Hostname)
}
