package monitor

import (
	"context"

	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/util/funcopt"
)

func WithRoutineTracer(o routinehelper.Tracer) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.SetTracer(o)
		return nil
	})
}

func WithContext(parent context.Context) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.ctx, t.cancel = context.WithCancel(parent)
		return nil
	})
}
