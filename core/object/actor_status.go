package object

import (
	"context"
	"sync"
	"time"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/util/hostname"
)

func (t *actor) FreshStatus(ctx context.Context) (instance.Status, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	return t.statusEval(ctx)
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
		t.log.Debug().Err(err).Msg("a status refresh is already in progress")
	}
}

func (t *actor) statusEval(ctx context.Context) (instance.Status, error) {
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedStatusEval(ctx)
}

func (t *actor) lockedStatusEval(ctx context.Context) (data instance.Status, err error) {
	data.App = t.App()
	data.Env = t.Env()
	data.Kind = t.path.Kind
	data.Updated = time.Now()
	data.Parents = t.Parents()
	data.Children = t.Children()
	data.DRP = t.config.IsInDRPNodes(hostname.Hostname())
	data.Subsets = t.subsetsStatus()
	data.Frozen = t.Frozen()
	data.Running = runningRIDList(t)
	if err = t.resourceStatusEval(ctx, &data); err != nil {
		return
	}
	if len(data.Resources) == 0 {
		data.Avail = status.NotApplicable
		data.Overall = status.NotApplicable
		data.Optional = status.NotApplicable
	}
	data.Csum = csumStatusData(data)
	t.statusDump(data)
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

func (t *actor) subsetsStatus() map[string]instance.SubsetStatus {
	data := make(map[string]instance.SubsetStatus)
	for _, rs := range t.ResourceSets() {
		if !rs.Parallel {
			continue
		}
		data[rs.Fullname()] = instance.SubsetStatus{
			Parallel: rs.Parallel,
		}
	}
	return data
}

func (t *actor) resourceStatusEval(ctx context.Context, data *instance.Status) error {
	resources := make([]resource.ExposedStatus, 0)
	var mu sync.Mutex
	return t.ResourceSets().Do(ctx, t, "", "status", func(ctx context.Context, r resource.Driver) error {
		xd := resource.GetExposedStatus(ctx, r)
		mu.Lock()
		resources = append(resources, xd)
		data.Resources = resources
		data.Overall.Add(xd.Status)
		if !xd.Optional {
			data.Avail.Add(xd.Status)
		}
		data.Provisioned.Add(xd.Provisioned.State)
		mu.Unlock()
		return nil
	})
}
