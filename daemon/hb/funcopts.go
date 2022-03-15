package hb

import (
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
