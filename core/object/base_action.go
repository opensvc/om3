package object

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/env"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/resourceset"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

type ActionOptioner interface {
	IsDryRun() bool
	GetResourceSelector() OptsResourceSelector
}

// Resources implementing setters
type (
	confirmer interface {
		SetConfirm(v bool)
	}
	forcer interface {
		SetForce(v bool)
	}
)

// Options structs implementing getters
type (
	isConfirmer interface {
		IsConfirm() bool
	}
	isForcer interface {
		IsForce() bool
	}
	isRollbackDisableder interface {
		IsRollbackDisabled() bool
	}
)

var (
	ErrInvalidNode = errors.New("invalid node")
)

func (t *Base) validateAction() error {
	if t.Env() != "PRD" && rawconfig.Node.Node.Env == "PRD" {
		return errors.Wrapf(ErrInvalidNode, "not allowed to run on this node (svc env=%s node env=%s)", t.Env(), rawconfig.Node.Node.Env)
	}
	if t.config.IsInNodes(hostname.Hostname()) {
		return nil
	}
	if t.config.IsInDRPNodes(hostname.Hostname()) {
		return nil
	}
	return errors.Wrapf(ErrInvalidNode, "hostname '%s' is not a member of DEFAULT.nodes, DEFAULT.drpnode nor DEFAULT.drpnodes", hostname.Hostname())
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

func (t *Base) needRollback(options ActionOptioner) bool {
	if options.(isRollbackDisableder).IsRollbackDisabled() {
		return false
	}
	k := key.Parse("disable_rollback")
	if t.Config().GetBool(k) {
		return false
	}
	return true
}

func (t *Base) action(action objectactionprops.T, options ActionOptioner, fn resourceset.DoFunc) error {
	if err := t.preAction(objectactionprops.Start, options); err != nil {
		return err
	}
	resourceSelector := options.GetResourceSelector()
	resourceLister := t.actionResourceLister(resourceSelector, action.Order)
	if err := t.ResourceSets().Do(resourceLister, resourceSelector.To, fn); err != nil {
		if t.needRollback(options) {
			t.Log().Err(err).Msg("")
			t.Log().Info().Msg("rollback")
			return fmt.Errorf("rollback not implemented")
		}
		return err
	}
	return nil
}

func (t *Base) notifyAction(action objectactionprops.T, options ActionOptioner) error {
	if env.HasDaemonOrigin() {
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

func (t *Base) setActionOptions(options interface{}) func() {
	for _, r := range t.Resources() {
		if r.IsDisabled() {
			continue
		}
		if a, ok := r.(forcer); ok {
			a.SetForce(options.(isForcer).IsForce())
		}
		if a, ok := r.(confirmer); ok {
			a.SetConfirm(options.(isConfirmer).IsConfirm())
		}
	}
	return t.unsetActionOptions
}

func (t *Base) unsetActionOptions() {
	for _, r := range t.Resources() {
		if r.IsDisabled() {
			continue
		}
		if a, ok := r.(forcer); ok {
			a.SetForce(false)
		}
		if a, ok := r.(confirmer); ok {
			a.SetConfirm(false)
		}
	}
}
