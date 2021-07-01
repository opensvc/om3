package object

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceselector"
)

// OptsStart is the options of the Start object method.
type OptsStart struct {
	OptsGlobal
	OptsAsync
	OptsLocking
	resourceselector.Options
	OptTo
	OptForce
	OptDisableRollback
}

// Start starts the local instance of the object
func (t *Base) Start(options OptsStart) error {
	ctx := actioncontext.New(options, objectactionprops.Start)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("start", false)
	defer t.postActionStatusEval(ctx)
	return t.lockedAction("", options.OptsLocking, "start", func() error {
		return t.lockedStart(ctx)
	})
}

func (t *Base) lockedStart(ctx context.Context) error {
	if err := t.abortStart(ctx); err != nil {
		return err
	}
	if err := t.masterStart(ctx); err != nil {
		return err
	}
	if err := t.slaveStart(ctx); err != nil {
		return err
	}
	return nil
}

func (t Base) abortWorker(ctx context.Context, r resource.Driver, q chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	a, ok := r.(resource.Aborter)
	if !ok {
		q <- false
		return
	}
	if a.Abort(ctx) {
		t.log.Error().Str("rid", r.RID()).Msg("abort start")
		q <- true
		return
	}
	q <- false
}

func (t *Base) abortStart(ctx context.Context) (err error) {
	t.log.Debug().Msg("abort start check")
	q := make(chan bool, len(t.Resources()))
	var wg sync.WaitGroup
	for _, r := range t.Resources() {
		if r.IsDisabled() {
			continue
		}
		wg.Add(1)
		go t.abortWorker(ctx, r, q, &wg)
	}
	wg.Wait()
	var ret bool
	for range t.Resources() {
		ret = ret || <-q
	}
	if ret {
		return errors.New("abort start")
	}
	return nil
}

func (t *Base) masterStart(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("start resource")
		return resource.Start(ctx, r)
	})
}

func (t *Base) slaveStart(ctx context.Context) error {
	return nil
}
