package object

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
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

func (t *actor) lockedStatusEval(ctx context.Context) (data instance.Status, err error) {
	t.setLastStartedAt(&data)
	data.UpdatedAt = time.Now()
	data.FrozenAt = t.Frozen()
	data.Running = runningRIDList(t)
	if err = t.resourceStatusEval(ctx, &data); err != nil {
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

func (t *actor) resourceStatusEval(ctx context.Context, data *instance.Status) error {
	data.Resources = make(instance.ResourceStatuses)
	var mu sync.Mutex
	err := t.ResourceSets().Do(ctx, t, "", "status", func(ctx context.Context, r resource.Driver) error {
		xd := resource.GetExposedStatus(ctx, r)

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
			data.Avail.Add(xd.Status)
		}
		data.Provisioned.Add(xd.Provisioned.State)
		mu.Unlock()
		return nil
	})
	t.Progress(ctx, colorstatus.Sprint(data.Avail, rawconfig.Colorize))
	return err
}
