package object

import (
	"context"
	"sync"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/timestamp"
)

// Status returns the service status dataset
func (t *actor) Status(options OptsStatus) (instance.Status, error) {
	var (
		data instance.Status
		err  error
	)
	ctx := context.Background()
	ctx = actioncontext.WithOptions(ctx, options)
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()

	if options.Refresh || t.statusDumpOutdated() {
		return t.statusEval(ctx, options)
	}
	if data, err = t.statusLoad(); err == nil {
		return data, nil
	}
	// corrupted status.json => eval
	return t.statusEval(ctx, options)
}

func (t *actor) postActionStatusEval(ctx context.Context) {
	if _, err := t.statusEval(ctx, OptsStatus{}); err != nil {
		t.log.Debug().Err(err).Msg("a status refresh is already in progress")
	}
}

func (t *actor) statusEval(ctx context.Context, options OptsStatus) (instance.Status, error) {
	props := actioncontext.Status
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedStatusEval(ctx)
}

func (t *actor) lockedStatusEval(ctx context.Context) (data instance.Status, err error) {
	data.App = t.App()
	data.Env = t.Env()
	data.Orchestrate = t.Orchestrate()
	data.Topology = t.Topology()
	data.Placement = t.Placement()
	data.Priority = t.Priority()
	data.Kind = t.path.Kind
	data.Updated = timestamp.Now()
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
	if data.Topology == topology.Flex {
		data.FlexTarget = t.FlexTarget()
		data.FlexMin = t.FlexMin()
		data.FlexMax = t.FlexMax()
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
	data.Resources = make(map[string]resource.ExposedStatus)
	var mu sync.Mutex
	return t.ResourceSets().Do(ctx, t, "", func(ctx context.Context, r resource.Driver) error {
		xd := resource.GetExposedStatus(ctx, r)
		mu.Lock()
		data.Resources[r.RID()] = xd
		data.Overall.Add(xd.Status)
		if !xd.Optional {
			data.Avail.Add(xd.Status)
		}
		data.Provisioned.Add(xd.Provisioned.State)
		mu.Unlock()
		return nil
	})
}
