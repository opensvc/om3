package subdaemon

import (
	"context"

	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/util/funcopt"
)

func WithContext(ctx context.Context) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.ctx, t.cancel = context.WithCancel(ctx)
		return nil
	})
}

func WithName(name string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.name = name
		return nil
	})
}

func WithMainManager(mgr Manager) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.main = mgr
		return nil
	})
}

func WithLogName(name string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.logName = name
		return nil
	})
}

func WithRoutineTracer(o routinehelper.Tracer) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.SetTracer(o)
		return nil
	})
}
