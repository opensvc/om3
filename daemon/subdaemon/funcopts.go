package subdaemon

import (
	"github.com/opensvc/om3/util/funcopt"
)

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
