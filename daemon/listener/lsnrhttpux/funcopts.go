package lsnrhttpux

import (
	"github.com/opensvc/om3/v3/util/funcopt"
)

func WithAddr(o string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.addr = o
		return nil
	})
}
