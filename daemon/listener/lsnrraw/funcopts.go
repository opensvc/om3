package lsnrraw

import (
	"net/http"

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

func WithHttpHandler(o http.Handler) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.httpHandler = o
		return nil
	})
}

func WithAddr(o string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.addr = o
		return nil
	})
}
