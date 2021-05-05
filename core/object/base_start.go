package object

import (
	"sync"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/ordering"
	"opensvc.com/opensvc/core/resource"
)

// OptsStart is the options of the Start object method.
type OptsStart struct {
	Global           OptsGlobal
	Async            OptsAsync
	Lock             OptsLocking
	ResourceSelector OptsResourceSelector
	Force            bool `flag:"force"`
}

// Start starts the local instance of the object
func (t *Base) Start(options OptsStart) error {
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("start", false)
	defer t.postActionStatusEval()
	return t.lockedAction("", options.Lock, "start", func() error {
		return t.lockedStart(options)
	})
}

func (t *Base) lockedStart(options OptsStart) error {
	if err := t.abortStart(options); err != nil {
		return err
	}
	if err := t.masterStart(options); err != nil {
		return err
	}
	if err := t.slaveStart(options); err != nil {
		return err
	}
	return nil
}

func (t Base) abortWorker(r resource.Driver, q chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	a, ok := r.(resource.Aborter)
	if !ok {
		q <- false
		return
	}
	if a.Abort() {
		t.log.Error().Str("rid", r.RID()).Msg("abort start")
		q <- true
		return
	}
	q <- false
}

func (t *Base) abortStart(options OptsStart) (err error) {
	t.log.Debug().Msg("abort start check")
	q := make(chan bool, len(t.Resources()))
	var wg sync.WaitGroup
	for _, r := range t.Resources() {
		wg.Add(1)
		go t.abortWorker(r, q, &wg)
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

func (t *Base) masterStart(options OptsStart) error {
	resourceLister := t.actionResourceLister(options.ResourceSelector)
	return t.ResourceSets().Do(resourceLister, ordering.Asc, func(r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("start resource")
		return resource.Start(r)
	})
}

func (t *Base) slaveStart(options OptsStart) error {
	return nil
}
