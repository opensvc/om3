package object

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/util/hostname"
)

func (t *actor) FreshStatus(ctx context.Context) (instance.Status, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	return t.statusEval(ctx)
}

// MonitorStatus returns the service status dataset with monitored resources
// refreshed and non-monitore resources loaded from cache
func (t *actor) MonitorStatus(ctx context.Context) (instance.Status, error) {
	var (
		data instance.Status
		err  error
	)
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	data, err = t.statusLoad()
	if err != nil {
		return t.FreshStatus(ctx)
	}
	return t.monitorStatusEval(ctx, data)
}

// Status returns the service status dataset
func (t *actor) Status(ctx context.Context) (instance.Status, error) {
	var (
		data instance.Status
		err  error
	)
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	if t.statusDumpOutdated() {
		return t.statusEval(ctx)
	}
	if data, err = t.statusLoad(); err == nil {
		return data, nil
	}
	// corrupted status.json => eval
	return t.statusEval(ctx)
}

func (t *actor) postActionStatusEval(ctx context.Context) {
	if _, err := t.statusEval(ctx); err != nil {
		t.log.Debugf("a status refresh is already in progress: %s", err)
	}
}

func (t *actor) monitorStatusEval(ctx context.Context, data instance.Status) (instance.Status, error) {
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedMonitorStatusEval(ctx, data)
}

func (t *actor) statusEval(ctx context.Context) (instance.Status, error) {
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedStatusEval(ctx)
}

func (t *actor) setLastStartedAt(data *instance.Status) error {
	stat, err := os.Stat(t.lastStartFile())
	switch {
	case errors.Is(err, os.ErrNotExist):
		data.LastStartedAt = time.Time{}
	case err != nil:
		return err
	default:
		data.LastStartedAt = stat.ModTime()
	}
	return nil
}

func (t *actor) lockedMonitorStatusEval(ctx context.Context, data instance.Status) (instance.Status, error) {
	t.setLastStartedAt(&data)
	data.UpdatedAt = time.Now()
	data.FrozenAt = t.Frozen()
	data.Running = runningRIDList(t)

	// reset fields that t.resourceStatusEval() will re-evaluate
	data.Avail = status.Undef
	data.Overall = status.Undef
	data.Provisioned = provisioned.Undef

	if err := t.resourceStatusEval(ctx, &data, true); err != nil {
		return data, err
	}
	if len(data.Resources) == 0 {
		data.Avail = status.NotApplicable
		data.Overall = status.NotApplicable
		data.Optional = status.NotApplicable
	}
	return data, t.statusDump(data)
}

func (t *actor) lockedStatusEval(ctx context.Context) (data instance.Status, err error) {
	t.setLastStartedAt(&data)
	data.UpdatedAt = time.Now()
	data.FrozenAt = t.Frozen()
	data.Running = runningRIDList(t)
	if err = t.resourceStatusEval(ctx, &data, false); err != nil {
		return
	}
	if len(data.Resources) == 0 {
		data.Avail = status.NotApplicable
		data.Overall = status.NotApplicable
		data.Optional = status.NotApplicable
	}
	err = t.statusDump(data)
	return
}

func runningRIDList(t interface{}) []string {
	l := make([]string, 0)
	for _, r := range listResources(t) {
		if i, ok := r.(resource.IsRunninger); !ok {
			continue
		} else if !i.IsRunning() {
			continue
		}
		l = append(l, r.RID())
	}
	return l
}

func (t *actor) isEncapNodeMatchingResource(r resource.Driver) (bool, error) {
	isEncapResource := r.IsEncap()
	isEncapNode, err := t.Config().IsInEncapNodes(hostname.Hostname())
	if err != nil {
		return false, err
	}
	if isEncapNode && isEncapResource {
		return true, nil
	}
	if !isEncapNode && !isEncapResource {
		return true, nil
	}
	return false, nil
}

func (t *actor) resourceStatusEval(ctx context.Context, data *instance.Status, monitoredOnly bool) error {
	if !monitoredOnly {
		data.Resources = make(instance.ResourceStatuses)
	}
	var mu sync.Mutex
	sb := statusbus.FromContext(ctx)
	err := t.ResourceSets().Do(ctx, t, "", "status", func(ctx context.Context, r resource.Driver) error {
		var xd resource.Status
		if v, err := t.isEncapNodeMatchingResource(r); err != nil {
			return err
		} else if !v {
			return nil
		}
		if monitoredOnly && !r.IsMonitored() {
			xd = data.Resources[r.RID()]
			sb.Post(r.RID(), xd.Status, false)
		} else {
			xd = resource.GetStatus(ctx, r)
		}

		// If the resource is up but the provisioned flag is unset, set
		// the provisioned flag.
		if xd.Provisioned.State == provisioned.False {
			switch xd.Status {
			case status.Up, status.StandbyUp:
				resource.SetProvisioned(ctx, r)
				xd.Provisioned.State = provisioned.True
			}
		}

		mu.Lock()
		data.Resources[r.RID()] = xd
		data.Overall.Add(xd.Status)
		if !xd.Optional {
			switch r.ID().DriverGroup() {
			case driver.GroupSync:
			case driver.GroupTask:
			default:
				data.Avail.Add(xd.Status)
			}
		}
		data.Provisioned.Add(xd.Provisioned.State)
		mu.Unlock()
		return nil
	})
	t.Progress(ctx, colorstatus.Sprint(data.Avail, rawconfig.Colorize))
	return err
}
