package hb

import (
	"github.com/opensvc/om3/daemon/routinehelper"
	"github.com/opensvc/om3/daemon/subdaemon"
	"github.com/opensvc/om3/util/funcopt"
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
