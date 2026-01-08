package lsnrhttpinet

import (
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/util/funcopt"
)

func WithAddr(o string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.addr = o
		return nil
	})
}

func WithCertFile(o string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.certFile = o
		return nil
	})
}

func WithKeyFile(o string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.keyFile = o
		return nil
	})
}

func WithRateLimiterConfig(o cluster.RateLimiterConfig) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.status.RateLimiter = o
		return nil
	})
}
