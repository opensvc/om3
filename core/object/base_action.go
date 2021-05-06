package object

import (
	"os"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
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

func (t *Base) notifyAction(action string, state string, dryRun bool, resourceSelector OptsResourceSelector) error {
	if config.HasDaemonOrigin() {
		return nil
	}
	if dryRun {
		return nil
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	req := c.NewPostObjectMonitor()
	req.ObjectSelector = t.Path.String()
	req.State = state
	req.LocalExpect = t.actionLocalExpect(action, resourceSelector)
	_, err = req.Do()
	return err
}

func (t *Base) mayFreeze(dryRun bool, resourceSelector OptsResourceSelector) error {
	if dryRun {
		t.log.Debug().Msg("skip freeze: dry run")
		return nil
	}
	if !resourceSelector.IsZero() {
		t.log.Debug().Msg("skip freeze: resource selection")
		return nil
	}
	if !t.orchestrateWantsFreeze() {
		t.log.Debug().Msg("skip freeze: orchestrate value")
		return nil
	}
	return t.Freeze()
}

func (t *Base) orchestrateWantsFreeze() bool {
	switch t.Orchestrate() {
	case "ha", "start":
		return true
	default:
		return false
	}
}

func (t *Base) actionLocalExpect(action string, resourceSelector OptsResourceSelector) string {
	if !resourceSelector.IsZero() {
		return ""
	}
	switch action {
	case "stop", "shutdown", "unprovision", "delete", "rollback", "toc":
		return ""
	default:
		return "unset"
	}
}
