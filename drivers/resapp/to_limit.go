package resapp

import (
	"opensvc.com/opensvc/util/capexec"
)

func (t T) toCaps() capexec.T {
	xo := capexec.T{}
	xo.LoadPG(t.PG)
	xo.LoadLimit(t.Limit)
	return xo
}
