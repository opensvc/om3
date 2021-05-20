package object

import (
	"os"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resourceset"
)

type ActionOptioner interface {
	IsDryRun() bool
	GetResourceSelector() OptsResourceSelector
}

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

func (t *Base) preAction(action objectactionprops.T, options ActionOptioner) error {
	if err := t.notifyAction(action, options); err != nil {
		t.Log().Debug().Err(err).Msgf("unable to notify %v preAction", action.Name)
	}
	if err := t.mayFreeze(action, options); err != nil {
		return err
	}
	return nil
}

func (t *Base) action(action objectactionprops.T, options ActionOptioner, fn resourceset.DoFunc) error {
	if err := t.preAction(objectactionprops.Start, options); err != nil {
		return err
	}
	resourceSelector := options.GetResourceSelector()
	resourceLister := t.actionResourceLister(resourceSelector, action.Order)
	if err := t.ResourceSets().Do(resourceLister, resourceSelector.To, fn); err != nil {
		return err
	}
	return nil
}

func (t *Base) notifyAction(action objectactionprops.T, options ActionOptioner) error {
	if config.HasDaemonOrigin() {
		return nil
	}
	if options.IsDryRun() {
		return nil
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	req := c.NewPostObjectMonitor()
	req.ObjectSelector = t.Path.String()
	req.State = action.Progress
	if options.GetResourceSelector().IsZero() {
		req.LocalExpect = action.LocalExpect
	}
	_, err = req.Do()
	return err
}

func (t *Base) mayFreeze(action objectactionprops.T, options ActionOptioner) error {
	if !action.Freeze {
		return nil
	}
	if options.IsDryRun() {
		t.log.Debug().Msg("skip freeze: dry run")
		return nil
	}
	if !options.GetResourceSelector().IsZero() {
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
