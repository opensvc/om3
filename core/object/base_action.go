package object

import (
	"os"

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

func (t *Base) setenv(action string, leader bool) {
	os.Setenv("OPENSVC_SVCPATH", t.Path.String())
	os.Setenv("OPENSVC_SVCNAME", t.Path.Name)
	os.Setenv("OPENSVC_NAMESPACE", t.Path.Namespace)
	os.Setenv("OPENSVC_ACTION", action)
	if leader {
		os.Setenv("OPENSVC_LEADER", "1")
	} else {
		os.Setenv("OPENSVC_LEADER", "0")
	}
	// each Setenv resource Driver will load its own env vars when actioned
}
