package listener

import (
	"context"

	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

func WithRoutineTracer(o routinehelper.Tracer) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.SetTracer(o)
		return nil
	})
}

func WithRootDaemon(o subdaemon.RootManager) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.rootDaemon = o
		return nil
	})
}

func WithContext(parent context.Context) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Ctx, t.CancelFunc = context.WithCancel(parent)
		return nil
	})
}
